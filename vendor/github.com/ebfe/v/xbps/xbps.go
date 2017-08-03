package xbps

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"

	"github.com/DHowett/go-plist"
	"xi2.org/x/xz"
)

type Package struct {
	Alternatives   map[string][]string `plist:"alternatives"`
	Arch           string              `plist:"architecture"`
	BuildDate      string              `plist:"build-date"`
	BuildOptions   string              `plist:"build-options"`
	ConfFiles      []string            `plist:"conf_files"`
	Conflicts      []string            `plist:"conflicts"`
	Sha256         string              `plist:"filename-sha256"`
	Size           uint64              `plist:"filename-size"`
	Homepage       string              `plist:"homepage"`
	InstallMsg     []byte              `plist:"install-msg"`
	InstalledSize  uint64              `plist:"installed_size"`
	License        string              `plist:"license"`
	Maintainer     string              `plist:"maintainer"`
	Pkgver         string              `plist:"pkgver"`
	Preserve       bool                `plist:"preserve"`
	Provides       []string            `plist:"provides"`
	Replaces       []string            `plist:"replaces"`
	Reverts        []string            `plist:"reverts"`
	Depends        []string            `plist:"run_depends"`
	ShlibProvides  []string            `plist:"shlib-provides"`
	ShlibRequires  []string            `plist:"shlib-requires"`
	ShortDesc      string              `plist:"short_desc"`
	SourceRevision string              `plist:"source-revisions"`
}

func ParseRepodata(r io.Reader) (*MetaData, Index, error) {
	var meta *MetaData
	var index Index
	var haveIndex = false

	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, err
	}

	ar := tar.NewReader(zr)
	for {
		hdr, err := ar.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		if hdr.Name == "index-meta.plist" {
			meta, err = parseMetaData(ar)
			if err != nil {
				return nil, nil, err
			}
		}
		if hdr.Name == "index.plist" {
			index, err = parseIndex(ar)
			if err != nil {
				return nil, nil, err
			}
			haveIndex = true
		}
	}

	if !haveIndex {
		return nil, nil, errors.New("xbps: missing index.plist in repodata")
	}
	return meta, index, nil
}

type Index map[string]Package

func parseIndex(r io.Reader) (Index, error) {
	var index Index

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	_, err = plist.Unmarshal(buf, &index)
	if err != nil {
		return nil, err
	}

	return index, nil
}

type MetaData struct {
	PubKey        []byte `plist:"public-key"`
	PubKeySize    int    `plist:"public-key-size"`
	SignatureBy   string `plist:"signature-by"`
	SignatureType string `plist:"signature-type"`
}

func parseMetaData(r io.Reader) (*MetaData, error) {
	var md MetaData

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// dummy index-meta.plist in local repos....
	if bytes.Equal([]byte("DEADBEEF"), buf) {
		return nil, nil
	}

	_, err = plist.Unmarshal(buf, &md)
	if err != nil {
		return nil, err
	}

	return &md, err
}

type Files struct {
	Dirs []struct {
		Name string `plist:"file"`
	} `plist:"dirs"`
	Files []struct {
		Name   string `plist:"file"`
		MTime  int    `plist:"mtime"`
		Sha256 string `plist:"sha256"`
	} `plist:"files"`
	Links []struct{
		Name   string `plist:"file"`
		MTime  int    `plist:"mtime"`
		Target string `plist:"target"`
	} `plist:"links"`
}

func ReadFiles(r io.Reader) (*Files, error) {
	ar, err := newXbpsReader(r)
	if err != nil {
		return nil, err
	}

	for {
		hdr, err := ar.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if hdr.Name == "./files.plist" {
			return parseFiles(ar)
		}
	}

	return nil, errors.New("xbps: no 'files.plist' in package")
}

func parseFiles(r io.Reader) (*Files, error) {
	var f Files

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	_, err = plist.Unmarshal(buf, &f)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func ReadProperties(r io.Reader) (*Package, error) {
	ar, err := newXbpsReader(r)
	if err != nil {
		return nil, err
	}
	for {
		hdr, err := ar.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if hdr.Name == "./props.plist" {
			return parseProperties(ar)
		}
	}

	return nil, errors.New("xbps: no 'files.plist' in package")
}

func parseProperties(r io.Reader) (*Package, error) {
	var f Package

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	_, err = plist.Unmarshal(buf, &f)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func newXbpsReader(r io.Reader) (*tar.Reader, error) {
	xr, err := xz.NewReader(r, 0)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(xr), nil
}
