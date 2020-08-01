package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/thijzert/vncreplay/rfb"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func main() {
	var inFile, outFile string
	var embedAssets bool
	flag.StringVar(&inFile, "i", "", "Input file")
	flag.StringVar(&outFile, "o", "replay.html", "Output file")
	flag.BoolVar(&embedAssets, "embedAssets", true, "Embed static assets in the output HTML")
	flag.Parse()

	if inFile == "" {
		if len(flag.Args()) > 0 {
			inFile = flag.Args()[0]
		} else {
			log.Fatalf("Usage: replay [-o OUTFILE] INFILE")
		}
	}

	out, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	replay, err := rfb.New(out)
	if err != nil {
		log.Fatal(err)
	}
	replay.EmbedAssets = embedAssets
	defer replay.Close()

	var handle *pcap.Handle

	// Open pcap file
	handle, err = pcap.OpenOffline(inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	var serverPort, sourcePort layers.TCPPort = 0, 0
	var serverSeq, clientSeq uint32 = 0, 0
	var t0 time.Time

	for packet := range packetSource.Packets() {
		// Get the TCP layer from this packet
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			// Get actual TCP data from this layer
			tcp, _ := tcpLayer.(*layers.TCP)

			meta := packet.Metadata()
			if serverPort == 0 && sourcePort == 0 {
				// Assume the first packet is the first SYN
				serverPort, sourcePort = tcp.DstPort, tcp.SrcPort
				t0 = meta.Timestamp
			}

			if tcp.SYN {
				if tcp.SrcPort == serverPort {
					serverSeq = tcp.Seq + 1
				} else if tcp.SrcPort == sourcePort {
					clientSeq = tcp.Seq + 1
				}
			}

			if len(tcp.Payload) == 0 {
				continue
			}

			tpacket := meta.Timestamp.Sub(t0)

			err = nil
			if tcp.SrcPort == serverPort {
				err = replay.ServerBytes(tpacket, int(tcp.Seq-serverSeq), tcp.Payload)
			} else if tcp.SrcPort == sourcePort {
				err = replay.ClientBytes(tpacket, int(tcp.Seq-clientSeq), tcp.Payload)
			} else {
				log.Printf("Ignoring extra traffic")
			}
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
