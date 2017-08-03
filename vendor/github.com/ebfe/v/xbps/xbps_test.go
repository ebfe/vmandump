package xbps

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func dump(v interface{}) {
	//pretty.Println(v)
}

func TestParseIndex(t *testing.T) {
	src, err := ioutil.ReadFile("_testdata/index.plist")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := parseIndex(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	dump(idx)
}

func TestParseFiles(t *testing.T) {
	src, err := ioutil.ReadFile("_testdata/files.plist")
	if err != nil {
		t.Fatal(err)
	}

	fl, err := parseFiles(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	dump(fl)
}

func TestParseProperties(t *testing.T) {
	src, err := ioutil.ReadFile("_testdata/props.plist")
	if err != nil {
		t.Fatal(err)
	}

	props, err := parseProperties(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	dump(props)
}
