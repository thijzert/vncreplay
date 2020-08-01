package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thijzert/go-resemble"
)

func main() {
	// Unpack source HTML template
	if err := os.Chdir("rfb/asset-src"); err != nil {
		log.Fatalf("Error: cannot find vncreplay asset source directory. (error: %s)\nAre you running this from the repository root?", err)
	}

	victrolaHTML, err := ioutil.ReadFile("victrola.html")
	if err != nil {
		log.Fatalf("Error loading HTML template: %s", err)
	}
	victrolaFragments, err := splitSnips(string(victrolaHTML), "stylesheet", "fake-readout", "fake-eventdump", "scripts")
	if err != nil {
		log.Fatalf("Error unpacking HTML template: %s", err)
	}

	// Stick the fragment between the readout and the event dump together - nothing goes between them
	victrolaFragments[1] += victrolaFragments[2]
	copy(victrolaFragments[2:], victrolaFragments[3:])
	victrolaFragments = victrolaFragments[:len(victrolaFragments)-1]

	os.Chdir("../..")

	// Embed vncreplay assets
	if err := os.Chdir("rfb/assets"); err != nil {
		log.Fatalf("Error: cannot find vncreplay assets directory. (error: %s)\nAre you running this from the repository root?", err)
	}

	for i := range victrolaFragments {
		f := fmt.Sprintf("victrola.unpacked.%d.html", i+1)
		err = ioutil.WriteFile(f, []byte(victrolaFragments[i]), 0644)
		if err != nil {
			log.Fatalf("Error writing unpacked HTML template: %s", err)
		}
	}

	var emb resemble.Resemble
	emb.OutputFile = "../assets.go"
	emb.PackageName = "rfb"
	emb.AssetPaths = []string{
		".",
	}
	if err := emb.Run(); err != nil {
		log.Fatal(err)
	}

	os.Chdir("../..")

	// Build main executable
	gofiles, err := filepath.Glob("cmd/vncreplay/*.go")
	if err != nil || gofiles == nil {
		log.Fatalf("Error: cannot find any go files to compile. (error: %s)", err)
	}
	compileArgs := append([]string{
		"build", "-o", "vncreplay",
	}, gofiles...)
	compile := exec.Command("go", compileArgs...)
	compile.Stdout = os.Stdout
	compile.Stderr = os.Stderr
	compile.Stdin = os.Stdin
	err = compile.Run()
	if err != nil {
		log.Fatalf("Compilation failed: %s", err)
	}
}

func splitSnips(input string, snips ...string) ([]string, error) {
	rv := make([]string, len(snips)+1)

	for i := range snips {
		sep := fmt.Sprintf("<!-- snip:%s -->", snips[i])
		parts := strings.Split(input, sep)
		if len(parts) != 3 {
			return nil, fmt.Errorf("syntax error: snip '%s' occurs %d times; must be exactly 2", snips[i], len(parts)-1)
		}
		rv[i] = parts[0]
		input = parts[2]
	}

	rv[len(snips)] = input
	return rv, nil
}
