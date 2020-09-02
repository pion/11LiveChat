
var log = msg => {
    console.log(msg)
}

var sock = null;
var wsuri = "wss://" + location.host + "/ws";
window.onload = function() {
    try {
        sock = new WebSocket(wsuri);

        sock.onmessage = function(e) {
            log("SDP chrome \<- sfu:\n" + e.data)

            // 2. receive websocket sdp which I am publish or subscribe

            // call setRemoteDescription

            var wsMsg = JSON.parse(e.data);

            if (wsMsg.Type == "publish") {
                window.processRcvSDPPublish(wsMsg.Sdp, wsMsg.name)
            }

            if (wsMsg.Type == "subscribe") {
                window.processRcvSDPSubscribe(wsMsg.Sdp, wsMsg.name)
            }
        }
        sock.onclose = function(e) {
            alert("closed")
        }
    } catch (e) {
        log(e)
    }
};


// click pub button
window.Pub = name => {
    let pcPublish = new RTCPeerConnection({
        // iceServers: [{
        //     urls: 'stun:stun.l.google.com:19302'
        // }]
    })


    // 0. getmedia and setLocalDescription
    navigator.mediaDevices.getUserMedia({ video: true, audio: true})
        .then(stream => {
            pcPublish.addStream(document.getElementById('local').srcObject = stream)
            pcPublish.createOffer()
                .then(d => pcPublish.setLocalDescription(d))
                .catch(log)
        }).catch(log)

    // 1. send publish sdp
    pcPublish.onicecandidate = event => {
        if (event.candidate === null) {
            log("SDP chrome ->  sfu:\n" + pcPublish.localDescription.sdp)
            var sendData = {type:'publish', sdp:pcPublish.localDescription.sdp, name:name}
            sock.send(JSON.stringify(sendData));
        }
    }


    // 3. receive sdp 
    window.processRcvSDPPublish = (sd,name) => {
        try {
            pcPublish.setRemoteDescription(new RTCSessionDescription({type:'answer', sdp:sd, name:name}))
        } catch (e) {
            log(e)
        }
    }

    // hide button
    // let btns = document.getElementsByClassName('sessbtn')
    // for (let i = 0; i < btns.length; i++) {
    //     btns[i].style = 'display: none'
    // }

    // show sdp info
    // document.getElementById('signalingContainer').style = 'display: block'
}


window.Sub = name => {
    let pcSubcribe = new RTCPeerConnection({
        // iceServers: [{
        //     urls: 'stun:stun.l.google.com:19302'
        // }]
    })


    // 1. send subscribe  sdp
    pcSubcribe.onicecandidate = event => {
        if (event.candidate === null) {
            log("SDP chrome ->  sfu:\n" + pcSubcribe.localDescription.sdp)
            var sendData = {type:'subscribe', sdp:pcSubcribe.localDescription.sdp, name:name}
            sock.send(JSON.stringify(sendData));
        }
    }

    pcSubcribe.addTransceiver('audio', {'direction': 'recvonly'})
    pcSubcribe.addTransceiver('video', {'direction': 'recvonly'})

    pcSubcribe.createOffer()
        .then(d => pcSubcribe.setLocalDescription(d))
        .catch(log)

    // 4. receive data
    pcSubcribe.ontrack = function (event) {
        var el = document.getElementById('remote')
        el.srcObject = event.streams[0]
        el.autoplay = true
        el.controls = true
    }

    // 3. receive sdp 
    window.processRcvSDPSubscribe = (sd, name) => {
        try {
            pcSubcribe.setRemoteDescription(new RTCSessionDescription({type:'answer', sdp:sd, name:name}))
        } catch (e) {
            log(e)
        }
    }
}

