package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"parser/pjson"
	"path/filepath"
	"strings"
)

// Parse
func parseFromKcp(o pjson.Object) *Package {
	p := new(Package)
	if s, e := o.GetString(NAME); e == nil {
		p.Name = s
	} else {
		return nil
	}
	if s, e := o.GetString(DESCRIPTION); e == nil {
		p.Description = s
	}
	if s, e := o.GetNumber(STARS); e == nil {
		p.Stars = int64(s)
	}
	return p
}

func parseFromDatabase(o pjson.Object) *Package {
	p := new(Package)
	if s, e := o.GetString(DB_NAME); e == nil {
		p.Name = s
	} else {
		return nil
	}
	if s, e := o.GetString(DB_DESCRIPTION); e == nil {
		p.Description = s
	}
	if s, e := o.GetString(DB_LOCALVERSION); e == nil {
		p.LocalVersion = s
	}
	if s, e := o.GetString(DB_KCPVERSION); e == nil {
		p.KcpVersion = s
	}
	if s, e := o.GetNumber(DB_STARS); e == nil {
		p.Stars = int64(s)
	}
	return p
}

// System operations
func launchCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func launchCommandWithResult(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.Output(); err == nil {
		return string(out), nil
	} else {
		return "", err
	}
}

// Pacman actions
func localVersion(app string) (v string, ok bool) {
	if r, e := launchCommandWithResult("pacman", "-Q", app); e == nil {
		f := strings.Fields(r)
		if len(f) >= 2 {
			v = f[1]
			ok = true
		}
	}
	return
}

func localAll(t_lst bool) PCollection {
	out := EmptyPCollection(t_lst)
	if r, e := launchCommandWithResult("pacman", "-Qm"); e == nil {
		for _, l := range strings.Split(r, "\n") {
			f := strings.Fields(l)
			if len(f) >= 2 {
				p := new(Package)
				p.Name = f[0]
				p.LocalVersion = f[1]
				out.Add(p)
			}
		}
	}
	return out
}

func localSearch(app string, t_lst bool) PCollection {
	out := EmptyPCollection(t_lst)
	if r, e := launchCommandWithResult("pacman", "-Qs", app); e == nil {
		rst := strings.Split(r, "\n")
		for i := 0; i < len(rst); i += 2 {
			f := strings.Fields(rst[i])
			if len(f) < 2 || !strings.HasPrefix(f[0], "local/") {
				continue
			}
			p := new(Package)
			p.Name = strings.TrimPrefix(f[0], "local/")
			p.LocalVersion = f[1]
			p.Description = strings.TrimSpace(rst[i+1])
			out.Add(p)
		}
	}
	return out
}

// I/O actions
func editPkgbuild() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = DEFAULT_EDITOR
	}
	return launchCommand(editor, "PKGBUILD")
}

func question(msg string, defaultValue bool) bool {
	var defstr string = "[Y/n]"
	if !defaultValue {
		defstr = "[y/N]"
	}
	fmt.Printf("\033[1;33m%s %s \033[m", msg, defstr)
	var response string
	if _, err := fmt.Scanf("%v", &response); err != nil || len(response) == 0 {
		return defaultValue
	}
	response = strings.ToLower(response)
	switch {
	case strings.HasPrefix(response, "y"):
		return true
	case strings.HasPrefix(response, "n"):
		return false
	default:
		return defaultValue
	}
}

// Dir operations
func pathJoin(dir, file string) string { return filepath.Join(dir, file) }

func pathOf(file string) string {
	pwd, _ := os.Getwd()
	return pathJoin(pwd, file)
}

func pathExists(path string) bool {
	_, e := os.Stat(path)
	return e == nil
}

func cd(dir string) { os.Chdir(dir) }

func userDir() string { return os.Getenv("HOME") }

// Git operations
func cloneRepo(app string) error { return launchCommand("git", "clone", fmt.Sprintf(URL_REPO, app)) }

// Install operations
func endInstall(wdir, lck string) {
	os.RemoveAll(wdir)
	os.Remove(lck)
}

// Remote operations
func launchRequest(debug bool, header string, searchbase string, v ...interface{}) ([]byte, error) {
	search := fmt.Sprintf(searchbase, v...)
	client := &http.Client{}
	request, err := http.NewRequest("GET", search, nil)
	if err != nil {
		return make([]byte, 0), err
	}
	if header != "" {
		request.Header.Add("Accept", header)
	}
	response, err := client.Do(request)
	if err != nil {
		return make([]byte, 0), err
	}
	if debug {
		response.Write(os.Stdout)
	}
	out, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	return out, err
}

func repoExists(app string) bool {
	_, e := launchRequest(false, "", URL_PKGBUILD, app)
	return e == nil
}

func searchError(o pjson.Object) error {
	msg, e1 := o.GetString(MESSAGE)
	doc, e2 := o.GetString(DOCUMENTATION)
	if e1 != nil || e2 != nil {
		return errors.New(Translate(MSG_UNKNOWN))
	}
	return errors.New(fmt.Sprintf("%s\n%s\n", msg, doc))
}

func remoteVersion(app string) (v string, ok bool) {
	r, e := launchRequest(false, "", URL_PKGBUILD, app)
	if e == nil {
		pkgbuild := string(r)
		pkgver, pkgrel := "", ""
		for _, l := range strings.Split(pkgbuild, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "pkgver=") {
				pkgver = l[7:]
			} else if strings.HasPrefix(l, "pkgrel=") {
				pkgrel = l[7:]
			}
			if pkgver != "" && pkgrel != "" {
				v = pkgver + "-" + pkgrel
				ok = true
				return
			}
		}
	}
	v = Translate(UNKNOWN_VERSION)
	return
}

func remoteAll(search string, debug, t_lst bool) (c PCollection, e error) {
	c = EmptyPCollection(t_lst)
	for i := 1; ; i++ {
		b, err := launchRequest(debug, HEADER, search, i, IDENT)
		if err != nil {
			e = err
			return
		}
		obj, err := pjson.ArrayObjectBytes(b)
		if err != nil {
			o, _ := pjson.ObjectBytes(b)
			e = searchError(o)
			return
		}
		if len(obj) == 0 {
			return
		}
		for _, o := range obj {
			p := parseFromKcp(o)
			if p != nil {
				p.LocalVersion, _ = localVersion(p.Name)
				c.Add(p)
			}
		}
	}
	return
}

func remoteSearch(app string, debug, t_lst bool) (c PCollection, e error) {
	c = EmptyPCollection(t_lst)
	b, err := launchRequest(debug, HEADERMATCH, SEARCH_APP, app, IDENT)
	if err != nil {
		e = err
		return
	}
	o, err := pjson.ObjectBytes(b)
	if err != nil {
		e = err
		return
	}
	items, err := o.GetArray(ITEMS)
	if err != nil {
		e = searchError(o)
		return
	}
	for _, v := range items {
		if o, e := v.Object(); e == nil {
			p := parseFromKcp(o)
			if p != nil {
				p.LocalVersion, _ = localVersion(p.Name)
				c.Add(p)
			}
		}
	}
	return
}

// Database management
func loadDB(dbpath string, t_lst bool) (c PCollection, e error) {
	c = EmptyPCollection(t_lst)
	var file *os.File
	file, e = os.Open(dbpath)
	if e == nil {
		var obj []pjson.Object
		obj, e = pjson.ArrayObjectReader(file)
		if e == nil {
			for _, o := range obj {
				p := parseFromDatabase(o)
				if p != nil {
					c.Add(p)
				}
			}
		}
	}
	return
}

func saveDB(dbpath string, c PCollection) error {
	obj := make([]pjson.Object, 0)
	for _, p := range c.List() {
		obj = append(obj, pjson.Object(p.Map()))
	}
	b, e := pjson.Marshal(obj)
	if e == nil {
		dbdir := filepath.Dir(dbpath)
		e = os.MkdirAll(dbdir, 0755)
		if e == nil {
			return ioutil.WriteFile(dbpath, b, 0644)
		}
	}
	return e
}
