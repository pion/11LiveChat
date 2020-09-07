package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"

	"github.com/davecgh/go-spew/spew"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			// URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
}

var (
	// Media engine
	m webrtc.MediaEngine

	// API object
	api *webrtc.API

	// Publisher Peer
	pubCount int32
	pcPub    *webrtc.PeerConnection

	// Local track
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}

	// Websocket upgrader
	upgrader = websocket.Upgrader{}

	// Broadcast channels
	broadcastHub = newHub()

	mediaInfo = make(map[string]avTrack)
)

const (
	rtcpPLIInterval = time.Second * 3
)

type wsMsg struct {
	Type          string
	Sdp           string
	Name          string
	CandidateInfo string
}

type avTrack struct {
	Video *webrtc.Track
	Audio *webrtc.Track
}

func sendWsMsg(c *websocket.Conn, mt int, dataToClient wsMsg) error {

	byteToClient, err := json.Marshal(dataToClient)

	if err != nil {
		return err
	}

	if err := c.WriteMessage(mt, byteToClient); err != nil {
		return err
	}

	return nil
}

func getAnswerSDP(pc *webrtc.PeerConnection) (string, error) {
	answer, err := pc.CreateAnswer(nil)

	if err != nil {
		return "", fmt.Errorf("create answer %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	// Sets the LocalDescription, and starts our UDP listeners
	if err := pc.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("SetLocalDescription %w", err)
	}
	<-gatherComplete

	sdpAnswer := pc.LocalDescription().SDP
	return sdpAnswer, nil
}

func getRcvMedia(name string, media map[string]avTrack) avTrack {
	if v, ok := media[name]; ok {
		return v
	}
	return avTrack{}
}

func ws(w http.ResponseWriter, r *http.Request) {

	log.Logger = log.With().Caller().Logger()
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)

	defer func() {
		checkError(c.Close())
	}()

	for {
		// Read sdp from websocket
		mt, msg, err := c.ReadMessage()
		checkError(err)

		wsData := wsMsg{}
		if err := json.Unmarshal(msg, &wsData); err != nil {
			checkError(err)
		}

		//TODO: record SDP to prometheus

		spew.Dump("<- SDP")
		spew.Dump("ws receive")

		sdp := wsData.Sdp
		name := wsData.Name

		m, ok := mediaInfo[name]

		if !ok {
			mediaInfo[name] = avTrack{}
			m = mediaInfo[name]
		}

		if wsData.Type == "publish" {

			// receive chrome publish sdp

			// Create a new RTCPeerConnection
			pcPub, err = api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeAudio)
			checkError(err)

			_, err = pcPub.AddTransceiver(webrtc.RTPCodecTypeVideo)
			checkError(err)

			// receive av data from chrome
			pcPub.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
				go func() {
					ticker := time.NewTicker(rtcpPLIInterval)
					for range ticker.C {
						if rtcpSendErr := pcPub.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
							checkError(rtcpSendErr)
						}
					}
				}()

				// v
				if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {
					// Create a local video track, all our SFU clients will be fed via this track
					var err error
					track, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
					checkError(err)

					m.Video = track
					mediaInfo[name] = m
					// spew.Dump(mediaInfo)
					// spew.Dump(track)
					// os.Exit(1)
					rtpBuf := make([]byte, 1400)
					for {
						i, err := remoteTrack.Read(rtpBuf)
						checkError(err)
						// videoTrackLock.RLock()
						_, err = track.Write(rtpBuf[:i])
						// videoTrackLock.RUnlock()

						if err != io.ErrClosedPipe {
							checkError(err)
						}
					}

				} else {
					// a

					// Create a local audio track, all our SFU clients will be fed via this track
					var err error
					audioTrack, err := pcPub.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
					checkError(err)

					m.Audio = audioTrack
					if _, ok := mediaInfo[name]; !ok {
						mediaInfo[name] = m
					}
					rtpBuf := make([]byte, 1400)
					for {
						i, err := remoteTrack.Read(rtpBuf)
						checkError(err)
						audioTrackLock.RLock()
						_, err = audioTrack.Write(rtpBuf[:i])
						audioTrackLock.RUnlock()
						if err != io.ErrClosedPipe {
							checkError(err)
						}
					}
				}
			})

			pcPub.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
				log.Info().Msgf("PUB ICE Connection State has changed: %s\n", connectionState.String())
			})

			pcPub.OnICECandidate(func(candi *webrtc.ICECandidate) {

				log.Info().Msgf("OnICECandidate %#v", candi)
				// spew.Dump(candi)

				// if candi == nil {
				// 	return
				// }

				// dataToClient := wsMsg{
				// 	Type:          "candidate",
				// 	CandidateInfo: candi.ToJSON().Candidate,
				// }
				// if err := sendWsMsg(c, mt, dataToClient); err != nil {
				// 	panic(err)
				// }
			})

			if err := pcPub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(sdp),
					Type: webrtc.SDPTypeOffer,
				}); err != nil {
				log.Error().Msgf("ERROR: %#v", err)
				continue
			}

			sdpAnswer, err := getAnswerSDP(pcPub)

			if err != nil {
				log.Error().Msgf("ERROR: %#v", err)
				continue
			}

			// Send server sdp to publisher
			dataToClient := wsMsg{
				Type: "publish",
				Sdp:  sdpAnswer,
				Name: name,
			}

			log.Info().Msg("-------------------Pub client SDP-------------")
			log.Info().Msg(sdp)
			log.Info().Msg("-------------------Pub server SDP-------------")
			log.Info().Msg(sdpAnswer)

			if err := sendWsMsg(c, mt, dataToClient); err != nil {
				log.Error().Msgf("ERROR: %#v", err)
			}
		}

		if wsData.Type == "subscribe" {
			m = getRcvMedia(name, mediaInfo)

			// spew.Dump(mediaInfo)
			// spew.Dump(name)

			pcSub, err := api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			// Waiting for publisher track finish
			// for {
			// 	if m.Audio == nil {
			// 		time.Sleep(100 * time.Millisecond)
			// 	} else {
			// 		break
			// 	}
			// }
			if m.Video != nil {
				_, err = pcSub.AddTrack(m.Video)
				checkError(err)
			}
			if m.Audio != nil {
				_, err = pcSub.AddTrack(m.Audio)
				checkError(err)
			}

			checkError(pcSub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(sdp),
					Type: webrtc.SDPTypeOffer,
				}))

			sdpAnswer, err := getAnswerSDP(pcSub)
			if err != nil {
				log.Error().Msgf("ERROR: %#v", err)
				continue
			}

			// Send sdp
			dataToClient := wsMsg{
				Type: "subscribe",
				// Sdp:  answer.SDP,
				Sdp:  sdpAnswer,
				Name: name,
			}

			if err := sendWsMsg(c, mt, dataToClient); err != nil {
				log.Error().Msgf("ERROR: %#v", err)
			}

		}
	}
}
