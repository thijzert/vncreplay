VNCreplay
=========

Developed as part of my solution to the 2020 Summerschool CTF, vncreplay replays a VNC session from captured network traffic.

It has some basic functionality to deal with missing or mangled packets, and outputs a standalone interactive player.

Building
--------
First, install dependencies using `go mod download`.
After that, use `go run build.go` to compile the project.

Usage
-----
The main executable takes an input and output file as its arguments.
The input file should be a PCAP file (of tcpdump or Wireshark fame), and the output file will be a standalone HTML file one can open in any modern browser.

For most use cases, this will do:

```bash
vncreplay -o player.html  path/to/capture.pcap
```

This will result in something resembling the following:

<p style="text-align: center">
	<img src="example.png" alt="Screenshot" style="width: 60%" />
</p>

License
-------
This program and its source code are available under the terms of the BSD 3-clause license.
Find out what that means here: https://www.tldrlegal.com/l/bsd3
