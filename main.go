package main

import (
	"bytes"
	"code.google.com/p/rsc/oauthprompt"
	"encoding/xml"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"text/template"
)

const (
	apiClientID     = "149464238275-cp5o4g6fp3immbv9k6pa6opobcfa1jeb.apps.googleusercontent.com"
	apiClientSecret = "28j_EJcB6w12x704OpHtlmwi"
	apiScope        = "https://www.googleapis.com/auth/userinfo.profile https://www.google.com/m8/feeds"
	tokenCache      = ".nokia-google-contacts-auth-cache"

	vcfContent = `BEGIN:VCARD
VERSION:2.1
FN:{{.Name}}
N:;{{.Name}};;;
{{range .Phones}}{{ printf "TEL;%s:%s\n" .Type .Number}}{{end}}END:VCARD`
)

var (
	client      = http.DefaultClient
	vcfTemplate = template.Must(template.New("vcf").Parse(vcfContent))
)

func main() {
	log.SetFlags(0)

	tr, err := oauthprompt.GoogleToken(tokenCache, apiClientID, apiClientSecret, apiScope)
	if err != nil {
		log.Fatal(err)
	}
	client = tr.Client()

	var f = "https://www.google.com/m8/feeds/contacts/default/full"
	var contacts []Contact

	for f != "" {
		r, err := client.Get(f)
		if err != nil {
			log.Fatal(err)
		}
		if r.StatusCode != http.StatusOK {
			log.Fatalf("Reading contact list: %v", r.Status)
		}
		data, err := ioutil.ReadAll(r.Body)

		feed := &Feed{}
		if err := xml.Unmarshal(data, &feed); err != nil {
			log.Fatalf("Parsing contact list: %v", err)
		}
		r.Body.Close()
		feed.Clean()
		for _, c := range feed.Contacts {
			if c.Name != "" && len(c.Phones) > 0 {
				contacts = append(contacts, c)
			}
		}
		f = feed.Next()
	}

	sort.Sort(byName(contacts))

	for _, c := range contacts {
		b := bytes.NewBuffer([]byte{})
		vcf, _ := os.Create(c.Name + ".vcf")
		vcfTemplate.Execute(b, c)
		vcf.Write(bytes.Replace(b.Bytes(), []byte("\n"), []byte("\r\n"), -1))
		vcf.Close()
	}
}

type byName []Contact

func (c byName) Len() int           { return len(c) }
func (c byName) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c byName) Less(i, j int) bool { return c[i].Name < c[j].Name }

type Feed struct {
	Links    []Link    `xml:"link"`
	Contacts []Contact `xml:"entry"`
}

func (f *Feed) Clean() {
	for i, c := range f.Contacts {
		for j, p := range c.Phones {
			if len(p.Type) > 34 {
				f.Contacts[i].Phones[j].Type = p.Type[33:]
			}
			switch f.Contacts[i].Phones[j].Type {
			case "mobile":
				f.Contacts[i].Phones[j].Type = "CELL"
			case "work":
				f.Contacts[i].Phones[j].Type = "WORK"
			default:
				f.Contacts[i].Phones[j].Type = "VOICE"
			}
		}
	}
}

func (f *Feed) Next() string {
	for _, l := range f.Links {
		if l.Where == "next" {
			return l.URL
		}
	}
	return ""
}

type Link struct {
	Where string `xml:"rel,attr"`
	URL   string `xml:"href,attr"`
}

type Contact struct {
	Name   string  `xml:"title"`
	Phones []Phone `xml:"http://schemas.google.com/g/2005 phoneNumber"`
}

type Phone struct {
	Type   string `xml:"rel,attr"`
	Number string `xml:",innerxml"`
}
