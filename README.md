# 11LiveChat - Just a DEMO

11LiveChat is a demo for a chatroom One to One. Basiclly from the examples `sfu-ws`.


# How to Run

* chrome1 open `https://x.x.x.x:8443/alice.html`
* click publish in chrome1
* chrome2 from another pc/laptop open `https://x.x.x.x:8443/bob.html`
* click subscribe in chrome2
* click publish in chrome2
* click subscribe in chrome1


# Arch

* The first join the room will send the data to server
* The send join the room will do a lot of things
    * receive the data and show on the receive window
    * publish the data to server
* The server will send the second joiner's data to publish, and show.
