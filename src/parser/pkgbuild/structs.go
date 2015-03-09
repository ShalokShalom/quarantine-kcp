package pkgbuild

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// --DECLARATION OF STRUCTURES -- //

// Data of a container
//  - Type of the data
//  - Line where the data is in the file (-1)
//  - Value of the data:
//      -> For variables' containers (var=...): one entry of the list, or all if no parenthesis
//      -> For other types: the complete line
type Data struct {
	Type  int
	Value string
	Line  int
}

// Container of data
//  - Type of container (variable, function, unknown variable, header, etc.)
//  - Begin, End: lines' range where the container is in the PKGBUILD
//  - Values: List of data the containers contains
//  - Name of container:
//      -> For variables: name of the variable
//      -> For function : name of the function
type Container struct {
	Type       int
	Name       string
	Begin, End int
	Values     []*Data
}

// Parsed PKGBUILD
type Pkgbuild map[string][]*Container

// -- CONSTRUCTORS -- //
func NewData(t int, v string, l int) *Data {
	return &Data{t, v, l}
}

func NewContainer(l string, i int) (*Container, int) {
	c := new(Container)
	c.Begin = i
	c.Values = make([]*Data, 0)
	t, e, d := lineType(l)
	switch t {
	case TD_VARIABLE:
		c.Name = d[1]
		c.Type = TC_UVARIABLE
		for _, v := range L_VARIABLES {
			if v == c.Name {
				c.Type = TC_VARIABLE
				break
			}
		}
		c.Append(t, i, splitString(d[2])...)
	case TD_FUNC:
		c.Name = d[0]
		c.Type = TC_UFUNCTION
		if strings.HasPrefix(d[1], "package_") {
			c.Type = TC_SFUNCTION
		} else {
			for _, v := range L_FUNCTIONS {
				if v == c.Name {
					c.Type = TC_FUNCTION
					break
				}
			}
		}
		c.Append(t, i, d[1])
	case TD_UNKNOWN:
		c.Name = UNKNOWN
		c.Type = TC_UNKNOWN
		c.Append(t, i, d[0])
	default:
		c.Name = BLANK
		c.Type = TC_BLANKCOMMENT
		c.Append(t, i, d[0])
	}
	return c, e
}

func NewPkgbuild() Pkgbuild {
	return make(Pkgbuild)
}

// -- STRING REPRESENTATION (for debug) -- //
func (d *Data) String() string {
	return d.Value
}

func (c *Container) String() string {
	return fmt.Sprintf("%s\n%s", c.Name, c.StringWithoutName())
}

func (p Pkgbuild) String() string {
	s := ""
	for k, cont := range p {
		if s != "" {
			s += "\n"
		}
		s = fmt.Sprintf("%s\033[1;31m%s\033[m", s, k)
		for _, c := range cont {
			s = fmt.Sprintf("%s\n-------------------------\n%s", s, c.StringWithoutName())
		}
		s += "\n-------------------------"
	}
	return s
}

func (d *Data) Quote() string {
	return quotify(d.Value)
}

func (c *Container) StringWithoutName() string {
	s := ""
	for _, d := range c.Values {
		if s != "" {
			s += "\n"
		}
		s = fmt.Sprintf("%s - '%s'", s, d.String())
	}
	return s
}

// -- UNPARSING -- //
func (c *Container) UnparseType() int {
	switch c.Type {
	case TC_VARIABLE:
		return U_VARIABLES[c.Name]
	case TC_UVARIABLE:
		return TU_OPTIONALQ
	default:
		return TU_LINES
	}
}

func (c *Container) Lines() []string {
	out := make([]string, 0)
	split, quote := true, true
	switch c.UnparseType() {
	case TU_SINGLEVAR:
		s := joinData(c.Values, !split, !quote)
		out = append(out, fmt.Sprintf("%s=%s", c.Name, s))
	case TU_SINGLEVARQ:
		s := joinData(c.Values, !split, !quote)
		out = append(out, fmt.Sprintf("%s=%s", c.Name, quotify(s)))
	case TU_OPTIONAL:
		if len(c.Values) > 1 {
			s := joinData(c.Values, split, quote)
			out = append(out, fmt.Sprintf("%s=(%s)", c.Name, s))
		} else {
			s := joinData(c.Values, !split, !quote)
			out = append(out, fmt.Sprintf("%s=%s", c.Name, s))
		}
	case TU_OPTIONALQ:
		s := joinData(c.Values, split, quote)
		if len(c.Values) > 1 {
			out = append(out, fmt.Sprintf("%s=(%s)", c.Name, s))
		} else {
			out = append(out, fmt.Sprintf("%s=%s", c.Name, s))
		}
	case TU_MULTIPLEVAR:
		s := joinData(c.Values, split, !quote)
		out = append(out, fmt.Sprintf("%s=(%s)", c.Name, s))
	case TU_MULTIPLEVARQ:
		s := joinData(c.Values, split, quote)
		out = append(out, fmt.Sprintf("%s=(%s)", c.Name, s))
	case TU_MULTIPLELINES:
		if len(c.Values) > 0 {
			s := c.Name + "=("
			t := strings.Repeat(" ", utf8.RuneCountInString(s))
			s += c.Values[0].Quote()
			for _, d := range c.Values[1:] {
				out = append(out, s)
				s = t + d.Quote()
			}
			out = append(out, s+")")
		} else {
			out = append(out, c.Name+"=()")
		}
	default:
		blank := false
		for _, d := range c.Values {
			l := d.String()
			if l != "" || !blank {
				out = append(out, l)
			}
			blank = l == ""
		}
	}
	return out
}

func (p Pkgbuild) Lines() []string {
	pc := NewPkgbuild()
	for _, c := range p {
		pc.Insert(c...)
	}
	o := pc.Order()
	lines := make([]string, 0)
	getLinesByKey(pc, o, HEADER, &lines)
	getLinesByKey(pc, o, PKGBASE, &lines)
	getLinesByKey(pc, o, PKGNAME, &lines)
	getLinesByKey(pc, o, PKGVER, &lines)
	getLinesByKey(pc, o, PKGREL, &lines)
	getLinesByKey(pc, o, EPOCH, &lines)
	getLinesByKey(pc, o, PKGDESC, &lines)
	getLinesByKey(pc, o, ARCH, &lines)
	getLinesByKey(pc, o, URL, &lines)
	getLinesByKey(pc, o, LICENSE, &lines)
	getLinesByKey(pc, o, GROUPS, &lines)
	getLinesByKey(pc, o, DEPENDS, &lines)
	getLinesByKey(pc, o, MAKEDEPENDS, &lines)
	getLinesByKey(pc, o, CHECKDEPENDS, &lines)
	getLinesByKey(pc, o, OPTDEPENDS, &lines)
	getLinesByKey(pc, o, PROVIDES, &lines)
	getLinesByKey(pc, o, CONFLICTS, &lines)
	getLinesByKey(pc, o, REPLACES, &lines)
	getLinesByKey(pc, o, BACKUP, &lines)
	getLinesByKey(pc, o, OPTIONS, &lines)
	getLinesByKey(pc, o, INSTALL, &lines)
	getLinesByKey(pc, o, CHANGELOG, &lines)
	getLinesByKey(pc, o, SOURCE, &lines)
	getLinesByKey(pc, o, NOEXTRACT, &lines)
	getLinesByKey(pc, o, MD5SUMS, &lines)
	getLinesByKey(pc, o, SHA1SUMS, &lines)
	getLinesByKey(pc, o, SHA256SUMS, &lines)

	getLinesByType(pc, o, TC_UVARIABLE, &lines)

	getLinesByKey(pc, o, PREPARE, &lines)
	getLinesByKey(pc, o, BUILD, &lines)
	getLinesByKey(pc, o, CHECK, &lines)
	getLinesByKey(pc, o, PACKAGE, &lines)

	getLinesByType(pc, o, TC_SFUNCTION, &lines)
	getLinesByType(pc, o, TC_UFUNCTION, &lines)

	getLinesByType(pc, o, TC_UNKNOWN, &lines)

	return lines
}

// -- MODIFICATION -- //

// Add data into a container
func (c *Container) Append(t, i int, v ...string) {
	for _, e := range v {
		c.Values = append(c.Values, NewData(t, e, i))
	}
}

// Modify a data of a container. If blank, remove it
func (c *Container) Set(idx int, v string) bool {
	if idx < 0 || idx >= len(c.Values) {
		return false
	}
	if v == "" {
		c.Values = append(c.Values[:idx], c.Values[idx+1:]...)
	} else {
		c.Values[idx].Value = v
	}
	return true
}

// Add a container in the parsed PKGBUILD
func (p Pkgbuild) Insert(cont ...*Container) {
	for _, c := range cont {
		if lc, ok := p[c.Name]; ok {
			p[c.Name] = append(lc, c)
		} else {
			p[c.Name] = []*Container{c}
		}
	}
}

// Remove a container of a PKGBUILD by its key and its position in array
func (p Pkgbuild) Remove(k string, i int) bool {
	if lc, ok := p[k]; ok {
		if i >= 0 && i < len(lc) {
			p[k] = append(lc[:i], lc[i+1:]...)
			return true
		}
	}
	return false
}

// -- SORTING CONTAINERS -- //
type LContainer []*Container

func (l LContainer) Len() int {
	return len(l)
}

func (l LContainer) Less(i, j int) bool {
	return l[i].Begin <= l[j].Begin
}

func (l LContainer) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l LContainer) Sort() {
	sort.Sort(l)
}

// Return all containers of the PKGBUILD, sorted by begin's line
func (p Pkgbuild) Sort() []*Container {
	out := make(LContainer, 0, len(p))
	for _, c := range p {
		out = append(out, c...)
	}
	out.Sort()
	return out
}

// Return map of previous container of a container
func (p Pkgbuild) Order() map[*Container]*Container {
	cs := p.Sort()
	out := make(map[*Container]*Container)
	if (len(cs)) > 1 {
		for i, c := range cs[1:] {
			out[c] = cs[i]
		}
	}
	return out
}

// -- OTHER -- //
func (c *Container) Empty() bool {
	return len(c.Values) == 0
}
