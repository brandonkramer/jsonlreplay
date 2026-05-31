package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/brandonkramer/jsonlreplay"
)

func main() {
	dir, err := os.MkdirTemp("", "jsonlreplay-example-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{CreateDir: true})
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	ev, err := w.AppendText("hello", "world")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("appended seq=%d time=%s\n", ev.Seq, ev.Time)

	events, err := jsonlreplay.Replay(path, 0, 10, jsonlreplay.ReadOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range events {
		fmt.Printf("%d %s %s\n", e.Seq, e.Kind, e.Text)
	}
}
