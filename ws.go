package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"

	// "github.com/pion/webrtc"
	"github.com/pion/webrtc/v2"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
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
	Type string
	Sdp  string
	Name string
}

type avTrack struct {
	Video *webrtc.Track
	Audio *webrtc.Track
}

// var c *websocket.Conn

func getRcvMedia(name string, media map[string]avTrack) avTrack {
	if v, ok := media[name]; ok {
		return v
	}
	return avTrack{}
}

func ws(w http.ResponseWriter, r *http.Request) {

	// Websocket client

	// var err1 error
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

		// spew.Dump("<- SDP")
		// spew.Dump("ws receive")

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

			// Set the remote SessionDescription
			checkError(pcPub.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(sdp),
					Type: webrtc.SDPTypeOffer,
				}))

			// Create answer
			answer, err := pcPub.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(pcPub.SetLocalDescription(answer))

			// Send server sdp to publisher
			dataToClient := wsMsg{
				Type: "publish",
				Sdp:  answer.SDP,
				Name: name,
			}

			byteToClient, err := json.Marshal(dataToClient)
			checkError(err)

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				checkError(err)
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

			answer, err := pcSub.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(pcSub.SetLocalDescription(answer))

			// Send sdp
			dataToClient := wsMsg{
				Type: "subscribe",
				Sdp:  answer.SDP,
				Name: name,
			}
			byteToClient, err := json.Marshal(dataToClient)
			checkError(err)

			if err := c.WriteMessage(mt, byteToClient); err != nil {
				checkError(err)
			}
		}
	}
}
