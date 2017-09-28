package main

import (
	"archive/tar"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebfe/v/xbps"
	"xi2.org/x/xz"
)

var outdir = "."

const (
	prefix        = "/usr/share/man/"
	statefilename = ".vmandump"
)

type state map[string][]string

func readState(path string) (state, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return nil, err
	}

	s := make(state)
	if err := json.Unmarshal(buf, &s); err != nil {
		return nil, err
	}

	return s, nil
}

func writeState(path string, s state) error {
	buf, err := json.MarshalIndent(&s, "", "\t")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, buf, 0644)
}

func matchFiles(files *xbps.Files) []string {
	m := []string{}
	for _, file := range files.Files {
		if strings.HasPrefix(file.Name, prefix) {
			m = append(m, file.Name)
		}
	}
	for _, link := range files.Links {
		if strings.HasPrefix(link.Name, prefix) {
			m = append(m, link.Name)
		}
	}
	return m
}

func extract(ar *tar.Reader, what map[string]struct{}) error {
	for len(what) != 0 {
		hdr, err := ar.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		fname := hdr.Name
		if strings.HasPrefix(fname, ".") {
			fname = fname[1:]
		}

		if _, ok := what[fname]; !ok {
			continue
		}
		delete(what, fname)

		outpath := filepath.Join(outdir, hdr.Name[len(prefix)+1:])
		if _, err = os.Stat(filepath.Dir(outpath)); os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(outpath), 0755)
		}
		fmt.Printf("\t%s\n", outpath)

		switch hdr.Typeflag {
		case tar.TypeSymlink:
			if filepath.Dir(hdr.Linkname) != "." {
				fmt.Fprintf(os.Stderr, "vmandump: skipping symlink: %q -> %q\n", outpath, hdr.Linkname)
				continue
			}
			err := os.Symlink(hdr.Linkname, outpath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vmandump: symlink: %q -> %q: %s\n", outpath, hdr.Linkname, err)
			}
		case tar.TypeLink:
			if !strings.HasPrefix(hdr.Linkname, "."+prefix) {
				fmt.Fprintf(os.Stderr, "vmandump: skipping hardlink: %q -> %q\n", outpath, hdr.Linkname)
				continue
			}
			target := filepath.Join(outdir, hdr.Linkname[len(prefix)+1:])
			err := os.Link(target, outpath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vmandump: %s\n", err)
			}
		case tar.TypeReg:
			f, err := os.Create(outpath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vmandump: %s\n", err)
				continue
			}
			_, err = io.Copy(f, ar)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vmandump: copy: %q: %s\n", outpath, err)
			}
			f.Close()
		default:
			fmt.Fprintf(os.Stderr, "vmandump: skipping unknown type %q at %q\n", hdr.Typeflag, hdr.Name)
		}
	}

	return nil
}

func process(base string, pkg *xbps.Package) ([]string, error) {
	pkgpath := filepath.Join(base, fmt.Sprintf("%s.%s.xbps", pkg.Pkgver, pkg.Arch))

	fpkg, err := os.Open(pkgpath)
	if err != nil {
		return nil, err
	}
	defer fpkg.Close()

	files, err := xbps.ReadFiles(fpkg)
	if err != nil {
		return nil, fmt.Errorf("read files.plist %q: %s\n", pkgpath, err)
	}

	manpages := matchFiles(files)
	if len(manpages) == 0 {
		return nil, nil
	}

	want := make(map[string]struct{}, len(manpages))
	for _, file := range manpages {
		want[file] = struct{}{}
	}

	fpkg.Seek(0, os.SEEK_SET)
	xr, err := xz.NewReader(fpkg, 0)
	if err != nil {
		return nil, err
	}
	ar := tar.NewReader(xr)

	fmt.Printf("%s.%s @ %s\n", pkg.Pkgver, pkg.Arch, pkg.Sha256)
	if err := extract(ar, want); err != nil {
		return nil, err
	}

	return manpages, nil
}

func main() {
	flag.StringVar(&outdir, "o", outdir, "output directory")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "usage: vmandump [-o outdir] /repo/$ARCH-repodata...\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	statefile := filepath.Join(outdir, statefilename)
	current, err := readState(statefile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vmandump: read state: %q: %s\n", statefile, err)
		os.Exit(1)
	}

	next := state{}
	for _, index := range flag.Args() {
		findex, err := os.Open(index)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vmandump: %s\n", err)
			os.Exit(1)
		}

		_, rindex, err := xbps.ParseRepodata(findex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vmandump: parse %q: %s\n", index, err)
			os.Exit(1)
		}

		base := filepath.Dir(index)
		for _, info := range rindex {
			if v, ok := current[info.Sha256]; ok {
				next[info.Sha256] = v
				continue
			}
			files, err := process(base, &info)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vmandump: %s\n", err)
				continue
			}
			next[info.Sha256] = files
		}
	}

	if err := writeState(statefile, next); err != nil {
		fmt.Fprintf(os.Stderr, "vmandump: write state: %q: %s\n", statefile, err)
		os.Exit(1)
	}
}
