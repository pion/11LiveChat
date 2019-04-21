package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	// ICEServers: []webrtc.ICEServer{
	// 	{
	// 		URLs: []string{"stun:stun.l.google.com:19302"},
	// 	},
	// },
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
}

var (
	// Media engine
	m webrtc.MediaEngine

	// API object
	api *webrtc.API

	// Publisher Peer
	pubCount    int32
	pubReceiver *webrtc.PeerConnection

	// Local track
	videoTrack     *webrtc.Track
	audioTrack     *webrtc.Track
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}

	// Websocket upgrader
	upgrader = websocket.Upgrader{}

	// Broadcast channels
	broadcastHub = newHub()
)

const (
	rtcpPLIInterval = time.Second * 3
)

type wsMsg struct {
	Type string
	Sdp  string
}

// var c *websocket.Conn

func ws(w http.ResponseWriter, r *http.Request) {

	// Websocket client

	// var err1 error
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)

	// defer func() {
	// 	checkError(c.Close())
	// }()

	// Read sdp from websocket
	mt, msg, err := c.ReadMessage()
	checkError(err)

	wsData := wsMsg{}
	if err := json.Unmarshal(msg, &wsData); err != nil {
		checkError(err)
	}

	//TODO: record SDP to prometheus

	// spew.Dump("<- SDP")
	spew.Dump(wsData)

	sdp := wsData.Sdp

	if wsData.Type == "publish" {

		// receive chrome publish sdp

		// Create a new RTCPeerConnection
		pubReceiver, err = api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		_, err = pubReceiver.AddTransceiver(webrtc.RTPCodecTypeAudio)
		checkError(err)

		_, err = pubReceiver.AddTransceiver(webrtc.RTPCodecTypeVideo)
		checkError(err)

		pubReceiver.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
			go func() {
				ticker := time.NewTicker(rtcpPLIInterval)
				for range ticker.C {
					if rtcpSendErr := pubReceiver.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
						checkError(rtcpSendErr)
					}
				}
			}()

			if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {

				// Create a local video track, all our SFU clients will be fed via this track
				var err error
				videoTrackLock.Lock()
				videoTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
				videoTrackLock.Unlock()
				checkError(err)

				rtpBuf := make([]byte, 1400)
				for {
					i, err := remoteTrack.Read(rtpBuf)
					checkError(err)
					videoTrackLock.RLock()
					_, err = videoTrack.Write(rtpBuf[:i])
					videoTrackLock.RUnlock()

					if err != io.ErrClosedPipe {
						checkError(err)
					}
				}

			} else {

				// Create a local audio track, all our SFU clients will be fed via this track
				var err error
				audioTrackLock.Lock()
				audioTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
				audioTrackLock.Unlock()
				checkError(err)

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
		checkError(pubReceiver.SetRemoteDescription(
			webrtc.SessionDescription{
				SDP:  string(sdp),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := pubReceiver.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(pubReceiver.SetLocalDescription(answer))

		// Send server sdp to publisher
		dataToClient := wsMsg{
			Type: "publish",
			Sdp:  answer.SDP,
		}

		byteToClient, err := json.Marshal(dataToClient)
		checkError(err)

		if err := c.WriteMessage(mt, byteToClient); err != nil {
			checkError(err)
		}

	}

	if wsData.Type == "subscribe" {

		// Create a new PeerConnection
		subSender, err := api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		// Waiting for publisher track finish
		for {
			videoTrackLock.RLock()
			if videoTrack == nil {
				videoTrackLock.RUnlock()
				//if videoTrack == nil, waiting..
				time.Sleep(100 * time.Millisecond)
			} else {
				videoTrackLock.RUnlock()
				break
			}
		}

		// Add local video track
		videoTrackLock.RLock()
		_, err = subSender.AddTrack(videoTrack)
		videoTrackLock.RUnlock()
		checkError(err)

		// Add local audio track
		audioTrackLock.RLock()
		_, err = subSender.AddTrack(audioTrack)
		audioTrackLock.RUnlock()
		checkError(err)

		// Set the remote SessionDescription
		checkError(subSender.SetRemoteDescription(
			webrtc.SessionDescription{
				SDP:  string(sdp),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := subSender.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(subSender.SetLocalDescription(answer))

		// Send sdp
		dataToClient := wsMsg{
			Type: "subscribe",
			Sdp:  answer.SDP,
		}
		byteToClient, err := json.Marshal(dataToClient)
		checkError(err)

		if err := c.WriteMessage(mt, byteToClient); err != nil {
			checkError(err)
		}
	}
}
