package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

func Test_Encode(t *testing.T) {
	s := encode("/Users/adrianloh/Desktop/gokali/src/kobako/suck.jpg")
	sr := strings.NewReader(s)
	ar := base64.NewDecoder(base64.StdEncoding, sr)
	buff := &bytes.Buffer{}
	wrote, err := io.Copy(buff, ar)
	if err != nil {
		t.Errorf(err.Error())
	} else if wrote != 68258 {
		t.Errorf("Expected to write 68258 bytes but wrote %d", wrote)
	}
}

/*


























*/
