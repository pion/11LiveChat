
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
                window.processRcvSDPPublish(wsMsg.Sdp)
            }

            // window.processRcvSDPSubscribe(e.data)
        }
    } catch (e) {
        log(e)
    }
};


// click pub button
window.Pub = () => {
    let pcPublish = new RTCPeerConnection({
        iceServers: [
        ]
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


            var sendData = {type:'publish', sdp:pcPublish.localDescription.sdp}
            sock.send(JSON.stringify(sendData));
        }
    }


    // 3. receive sdp 
    window.processRcvSDPPublish = (sd) => {
        try {
            pcPublish.setRemoteDescription(new RTCSessionDescription({type:'answer', sdp:sd}))
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


window.Sub = () => {
    let pcSubcribe = new RTCPeerConnection({
        iceServers: [
        ]
    })


    // 1. send subscribe  sdp
    pcSubcribe.onicecandidate = event => {
        if (event.candidate === null) {
            document.getElementById('publishSDP').value = pcSubcribe.localDescription.sdp;
            sock.send(pcSubcribe.localDescription.sdp);
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
    window.processRcvSDPSubscribe = (sd) => {
        try {
            pcSubcribe.setRemoteDescription(new RTCSessionDescription({type:'answer', sdp:sd}))
        } catch (e) {
            log(e)
        }
    }
}

