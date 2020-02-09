package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

type WsMsg struct {
	Tp  string `json:"tp"` //msg,sdp,online
	Val string `json:"val"`
	Id string `json:"id"`
}

type SysUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	UserAccount string `json:"user_account"`
	DepID       string `json:"dep_id"`
	ParentID    string `json:"parent_id"`
}
type Conference struct {
	ID           string `json:"id" gorm:"primary_key"`
	Name         string `json:"name"` //会议名称
	Bz           string `json:"bz"`   //参会范围:0-全局;1-局域
	Username     string `json:"username"`
	UserAccount  string `json:"user_account"`
	DepID        string `json:"dep_id"`
	Flag         int    `json:"flag"` // 0创建，1加入，2桥接
	CreationTime string `json:"creation_time"`
}
type Meeting struct {
	con *Conference
	p   []*SysUser
}

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs:       []string{"stun:129.211.114.37:3478"},
			Username:   "kurento",
			Credential: "kurento",
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
	pubCount    int32
	pubReceiver *webrtc.PeerConnection

	// Local track
	videoTrack     *webrtc.Track
	audioTrack     *webrtc.Track
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}
	hubLock = sync.RWMutex{}
	// Websocket upgrader
	upgrader   = websocket.Upgrader{}
	//meeting information
	meetingHub = make(map[string]*Meeting)
	//webrtc broadcast hub according meeting id
	broadcastHubs = make(map[string]*BroadcastHub)
	//websocket session hub
	websocketHub = make(map[*websocket.Conn]string)
	// Broadcast channels
	//broadcastHub = newHub()
)

const (
	rtcpPLIInterval = time.Second * 3
)
/*
func room(w http.ResponseWriter, r *http.Request) {

	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)
	defer func() {
		checkError(c.Close())
	}()
	// Read sdp from websocket
	mt, msg, err := c.ReadMessage()
	checkError(err)

	if atomic.LoadInt32(&pubCount) == 0 {
		atomic.AddInt32(&pubCount, 1)

		// Create a new RTCPeerConnection
		pubReceiver, err = api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		_, err = pubReceiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
		checkError(err)

		//Create DataChannel
		_, err = pubReceiver.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
		checkError(err)

		pubReceiver.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
			if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {

				// Create a local video track, all our SFU clients will be fed via this track
				var err error
				videoTrackLock.Lock()
				videoTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
				videoTrackLock.Unlock()
				checkError(err)

				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(rtcpPLIInterval)
					for range ticker.C {
						checkError(pubReceiver.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: videoTrack.SSRC()}}))
					}
				}()

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
				SDP:  string(msg),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := pubReceiver.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(pubReceiver.SetLocalDescription(answer))

		// Send server sdp to publisher
		checkError(c.WriteMessage(mt, []byte(answer.SDP)))

		// Register incoming channel
		pubReceiver.OnDataChannel(func(d *webrtc.DataChannel) {
			fmt.Println("data channel coming...")
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				// Broadcast the data to subSenders
				broadcastHub.broadcastChannel <- msg.Data
			})
		})
	} else {

		// Create a new PeerConnection
		subSender, err := api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		// Register data channel creation handling
		subSender.OnDataChannel(func(d *webrtc.DataChannel) {
			broadcastHub.addListener(d)
		})

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
				SDP:  string(msg),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := subSender.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(subSender.SetLocalDescription(answer))

		// Send server sdp to subscriber
		checkError(c.WriteMessage(mt, []byte(answer.SDP)))
	}
}
*/
func publisher(w http.ResponseWriter, r *http.Request) {
	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)
	//defer func() {
	//	checkError(c.Close())
	//}()
	// Read sdp from websocket
	for {

		mt, msg, err := c.ReadMessage()
		checkError(err)
		var wsMsg= WsMsg{}
		checkError(json.Unmarshal(msg, &wsMsg))
		log.Printf("coming websocket msg:%s \n", string(msg))
		if wsMsg.Tp == "msg" {
			var meet= &Meeting{}
			checkError(json.Unmarshal([]byte(wsMsg.Val), &meet.con))
			hubLock.Lock()
			websocketHub[c] = wsMsg.Id
			if _, ok := meetingHub[wsMsg.Id]; ok {
				meet.p = nil
			}
			u := &SysUser{UserAccount: meet.con.UserAccount, Username: meet.con.Username, DepID: meet.con.DepID}
			meet.p = append(meet.p, u)
			meetingHub[wsMsg.Id] = meet
			broadcastHubs[wsMsg.Id] = newHub()
			hubLock.Unlock()
		} else {
			broadcastHub := broadcastHubs[wsMsg.Id]
			// Create a new RTCPeerConnection
			pubReceiver, err = api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			_, err = pubReceiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
			checkError(err)

			//Create DataChannel
			_, err = pubReceiver.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
			checkError(err)

			pubReceiver.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
				if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {

					// Create a local video track, all our SFU clients will be fed via this track
					var err error
					videoTrackLock.Lock()
					videoTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
					videoTrackLock.Unlock()
					checkError(err)

					// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
					go func() {
						ticker := time.NewTicker(rtcpPLIInterval)
						for range ticker.C {
							checkError(pubReceiver.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: videoTrack.SSRC()}}))
						}
					}()

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
					SDP:  string(msg),
					Type: webrtc.SDPTypeOffer,
				}))

			// Create answer
			answer, err := pubReceiver.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(pubReceiver.SetLocalDescription(answer))

			// Send server sdp to publisher
			checkError(c.WriteMessage(mt, []byte(answer.SDP)))

			// Register incoming channel
			pubReceiver.OnDataChannel(func(d *webrtc.DataChannel) {
				fmt.Println("data channel coming...")
				d.OnMessage(func(msg webrtc.DataChannelMessage) {
					// Broadcast the data to subSenders
					broadcastHub.broadcastChannel <- msg.Data
				})
			})
		}
	}
}

func subcriber(w http.ResponseWriter, r *http.Request) {
	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)
	//defer func() {
	//	checkError(c.Close())
	//}()
	// Read sdp from websocket
	for {
		mt, msg, err := c.ReadMessage()
		checkError(err)
		var wsMsg= WsMsg{}
		checkError(json.Unmarshal(msg, &wsMsg))
		if wsMsg.Tp == "msg" {
			var meet= &Meeting{}
			checkError(json.Unmarshal([]byte(wsMsg.Val), &meet.con))
			u := &SysUser{UserAccount: meet.con.UserAccount, Username: meet.con.Username, DepID: meet.con.DepID}
			hubLock.Lock()
			websocketHub[c] = wsMsg.Id
			if m, ok := meetingHub[wsMsg.Id]; ok {
				m.p = append(m.p, u)
			}
			hubLock.Unlock()
			//broadcast this meet online
			for conn, meet := range websocketHub {
				m := WsMsg{Tp: "online", Val: strconv.Itoa(len(meetingHub[wsMsg.Id].p)), Id: wsMsg.Id}
				ms, err := json.Marshal(m)
				checkError(err)
				if meet == wsMsg.Id {
					checkError(conn.WriteMessage(mt, ms))
				}
			}
		} else {
			//get the meeting broadcastHub
			broadcastHub := broadcastHubs[wsMsg.Id]
			// Create a new PeerConnection
			subSender, err := api.NewPeerConnection(peerConnectionConfig)
			checkError(err)

			// Register data channel creation handling
			subSender.OnDataChannel(func(d *webrtc.DataChannel) {
				broadcastHub.addListener(d)
			})

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
					SDP:  string(msg),
					Type: webrtc.SDPTypeOffer,
				}))

			// Create answer
			answer, err := subSender.CreateAnswer(nil)
			checkError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			checkError(subSender.SetLocalDescription(answer))

			// Send server sdp to subscriber
			checkError(c.WriteMessage(mt, []byte(answer.SDP)))
		}
	}
}