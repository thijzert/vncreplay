package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/thijzert/go-resemble"
)

func main() {
	// Embed vncreplay assets
	if err := os.Chdir("rfb/assets"); err != nil {
		log.Fatalf("Error: cannot find vncreplay assets directory. (error: %s)\nAre you running this from the repository root?", err)
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
