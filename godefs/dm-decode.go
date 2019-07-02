package godefs

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

type DMObject struct {
	Data     interface{}
	DataSize int
	Extra    []byte
}

func (mp *DMObject) Decode(eventData map[string]interface{}) error {


	// TODO: assert string
	structdata, err := base64.StdEncoding.DecodeString(eventData["data"].(string))
	if err != nil {
		fmt.Printf("ERROR decoding base64 event data: %s", err)
		return errors.New("base64 decode fail")
	}

	buf := bytes.NewReader(structdata)
	err = binary.Read(buf, binary.LittleEndian, mp.Data)

	mp.DataSize = binary.Size(mp.Data)

	if mp.DataSize < len(structdata) {
		mp.Extra = structdata[mp.DataSize:]
	}

	return err
}

func (mp *DMObject) GetStringFromOffset(offset int) (string, error) {
	arrayIndex := offset - mp.DataSize
	if arrayIndex > len(mp.Extra) || arrayIndex < 0 {
		return "", errors.New("Attempt to access out of bounds data")
	}
	stringEnd := bytes.IndexByte(mp.Extra[arrayIndex:], 0) + arrayIndex
	if stringEnd > len(mp.Extra)-1 {
		stringEnd = len(mp.Extra) - 1
	}
	return string(mp.Extra[arrayIndex:stringEnd]), nil
}
