var sock = null;
var wsuri = "wss://" + location.host + "/ws";
window.onload = function() {
    try {
        sock = new WebSocket(wsuri);

        // 3. when receive websocket sdp, save it to textfield
        sock.onmessage = function(e) {
            // 如果不需要展示的话就不用写到 textarea 里面
            // document.getElementById('SDPReceive').value = e.data

            // call setRemoteDescription
         window.startSession(e.data)
        }
    } catch (e) {
    }
};


// click pub button
window.Pub = () => {
    let pcPublish = new RTCPeerConnection({
        iceServers: [
        ]
    })

    console.log("pub")

    // 1. getmedia and setLocalDescription
    navigator.mediaDevices.getUserMedia({ video: true, audio: true})
        .then(stream => {
            pcPublish.addStream(document.getElementById('local').srcObject = stream)
            pcPublish.createOffer()
                .then(d => pcPublish.setLocalDescription(d))
                .catch(log)
        }).catch(log)

    // 2. send publish sdp when ice done
    pcPublish.onicecandidate = event => {
        if (event.candidate === null) {
            document.getElementById('publishSDP').value = pcPublish.localDescription.sdp;
            sock.send(pcPublish.localDescription.sdp);
        }
    }


    // 4. receive sdp then called
    window.startSession = (sd) => {
        // 直接传參进来就好了
        // let sd = document.getElementById('SDPReceive').value
        if (sd === '') {
            return alert('Session Description must not be empty')
        }

        try {
            pcPublish.setRemoteDescription(new RTCSessionDescription({type:'answer', sdp:sd}))
        } catch (e) {
            alert(e)
        }
    }

    // hide button
    // let btns = document.getElementsByClassName('sessbtn')
    // for (let i = 0; i < btns.length; i++) {
    //     btns[i].style = 'display: none'
    // }

    // show sdp info
    document.getElementById('signalingContainer').style = 'display: block'
}


window.Sub = () => {
    let pcSubcribe = new RTCPeerConnection({
        iceServers: [
        ]
    })


    // 1 send publish sdp
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

        pcSubcribe.ontrack = function (event) {
            var el = document.getElementById('remote')
            el.srcObject = event.streams[0]
            el.autoplay = true
            el.controls = true
        }
}

