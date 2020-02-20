package rawjsonstream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"

	"github.com/superchalupa/sailfish/src/httpinject"
)

type busComponents interface {
	GetBus() eh.EventBus
}

type inputhandler interface {
	GetCommandCh() chan *httpinject.InjectCommand
}

func StartPipeHandler(logger log.Logger, pipePath string, d busComponents, s inputhandler) {
	err := makeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating UDB pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe", "err", err)
	}

	defer file.Close()

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
	}

	// defer .Close() to keep linters happy. Inside we know we never exit...
	defer nullWriter.Close()

	seq := int64(0)

	// there isn't a great way to re-sync the scanner if we fail a decode (syntax error or other),
	// so the easiest to code alternative is the one-json-object-per-line model. No newlines allowed inside json.

	scanner := bufio.NewReader(file)
outer:
	for {
		readLine := true
		line := []byte{}
		for readLine {
			raw_line, isPrefix, err := scanner.ReadLine()
			readLine = isPrefix
			line = append(line, raw_line...)
			if err == io.EOF {
				fmt.Printf("Got EOF while reading from pipe")
				break outer // should never happen
			}
		}
		fmt.Printf("Raw line      : %s\n", line)

		cmd := httpinject.NewInjectCommand()
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(cmd)

		if err != nil {
			fmt.Printf("error decoding stream json: %s\n", err)
			continue
		}

		cmd.SetPumpSendTime()
		cmd.Barrier = true
		cmd.Synchronous = true
		if cmd.Name == "" {
			fmt.Printf("No name specified. dropping: %+v\n", cmd)
			continue
		}

		cmd.Add(1)
		cmd.EventSeq = seq
		seq++
		fmt.Printf("Send to ch(%d): %+v\n", cmd.EventSeq, cmd)
		s.GetCommandCh() <- cmd
		fmt.Printf("Check on q : %d\n", cmd.EventSeq)
		success := <-cmd.GetResCh()
		fmt.Printf("Waiting for: %d - %t\n", cmd.EventSeq, success)
		cmd.Wait()
		fmt.Printf("DONE       : %d - %t\n", cmd.EventSeq, success)
	}
}
