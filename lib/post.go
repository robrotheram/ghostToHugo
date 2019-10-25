package ghostToHugo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	"github.com/gohugoio/hugo/hugolib"
	"github.com/jbarone/mobiledoc"
	"github.com/spf13/viper"
)

type mobiledocCard struct {
	Name     string `json:"cardName"`
	Markdown string `json:"markdown"`
}

type post struct {
	ID              json.RawMessage `json:"id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Content         string          `json:"markdown"`
	Plain           string          `json:"plaintext"`
	MobileDoc       string          `json:"mobiledoc,omitempty"`
	Image           string          `json:"image"`
	FeaturedImage   string          `json:"feature_image,omitempty"`
	Page            json.RawMessage `json:"page"`
	Status          string          `json:"status"`
	MetaDescription string          `json:"meta_description"`
	AuthorID        json.RawMessage `json:"author_id"`
	PublishedAt     json.RawMessage `json:"published_at"`
	CreatedAt       json.RawMessage `json:"created_at"`

	Published time.Time
	Created   time.Time
	IsDraft   bool
	IsPage    bool
	Author    string
	Tags      []string
}

func atomSoftReturn(value string, payload interface{}) string {
	return "\n"
}

func cardHTML(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	if html, ok := m["html"]; ok {
		return html.(string)
	}

	return ""
}

func cardMarkdown(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}
	if markdown, ok := m["markdown"]; ok {
		return fmt.Sprintf("%s\n", markdown.(string))
	}
	return ""
}

func cardImage(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	src, ok := m["src"]
	if !ok {
		log.Println("ERROR image card missing source")
		return ""
	}

	if caption, ok := m["caption"]; ok {
		return fmt.Sprintf(
			"{{< figure src=\"%s\" caption=\"%s\" >}}\n",
			src,
			caption,
		)
	}

	return fmt.Sprintf("{{< figure src=\"%s\" >}}\n", src)
}

func cardGallery(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("<figure>\n")
	buf.WriteString("  <div>\n")   // gallery container
	buf.WriteString("    <div>\n") // start of first row
	images, ok := m["images"]
	if !ok {
		return ""
	}

	for i, img := range images.([]interface{}) {
		if i > 0 && i%3 == 0 {
			buf.WriteString("    </div>") // end of current row
			buf.WriteString("    <div>")  // start of next row
		}
		image, ok := img.(map[string]interface{})
		if !ok {
			continue
		}
		buf.WriteString("      <div>")
		buf.WriteString("<img")
		buf.WriteString(fmt.Sprintf(" src=%q", stripContentFolder(image["src"].(string))))
		buf.WriteString(fmt.Sprintf(" width=\"%.0f\"", image["width"].(float64)))
		buf.WriteString(fmt.Sprintf(" height=\"%.0f\"", image["height"].(float64)))
		if alt, ok := image["alt"]; ok {
			buf.WriteString(fmt.Sprintf(" alt=%q", alt.(string)))
		}
		if title, ok := image["title"]; ok {
			buf.WriteString(fmt.Sprintf(" title=%q", title.(string)))
		}
		buf.WriteString("/>")
		buf.WriteString("</div>\n")
	}
	buf.WriteString("    </div>\n") // end of current row
	buf.WriteString("  </div>\n")   // end of gallery container

	if caption, ok := m["caption"]; ok {
		buf.WriteString(fmt.Sprintf("  <figcaption>\n    %s\n  </figcaption>\n", caption.(string)))
	}

	buf.WriteString("</figure>")

	return buf.String()
}

func cardHR(payload interface{}) string {
	return "---\n"
}

func cardCode(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	var buf bytes.Buffer

	buf.WriteString("```")
	if lang, ok := m["language"]; ok {
		buf.WriteString(lang.(string))
	}
	buf.WriteString("\n")
	buf.WriteString(m["code"].(string))
	buf.WriteString("\n```\n")

	return buf.String()
}

func cardBookmark(payload interface{}) string {
	/*
	 */
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	meta, ok := m["metadata"]
	if !ok {
		return ""
	}

	metadata, ok := meta.(map[string]interface{})
	if !ok {
		return ""
	}

	url, ok := metadata["url"]
	if !ok {
		return ""
	}
	title, ok := metadata["title"]
	if !ok {
		return ""
	}
	description, ok := metadata["description"]
	if !ok {
		return ""
	}

	var thumbnail, icon, author, publisher, caption string
	thumb, ok := metadata["thumbnail"]
	if ok {
		thumbnail = fmt.Sprintf("<div><img src=%q></div>", thumb.(string))
	}
	iconinfo, ok := metadata["icon"]
	if ok {
		icon = fmt.Sprintf("<img src=%q>", iconinfo.(string))
	}
	authorinfo, ok := metadata["author"]
	if ok && authorinfo != nil {
		author = fmt.Sprintf("<span>%s</span>", authorinfo.(string))
	}
	publisherinfo, ok := metadata["publisher"]
	if ok && publisherinfo != nil {
		publisher = fmt.Sprintf("<span>%s</span>", publisherinfo.(string))
	}

	capt, ok := m["caption"]
	if ok && capt != nil {
		caption = fmt.Sprintf("<figcaption>%s</figcaption>", capt.(string))
	}

	return fmt.Sprintf(
		`<figure>
	     <a href=%q>
	       <div>
	         <div>%s</div>
	         <div>%s</div>
	         <div>
	           %s
	           %s
	           %s
	         </div>
	       </div>
	       %s
	     </a>
	     %s
	   </figure>`,
		url.(string),
		title.(string),
		description.(string),
		icon,
		author,
		publisher,
		thumbnail,
		caption,
	)
}

func cardEmbed(payload interface{}) string {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return ""
	}

	html, ok := m["html"]
	if !ok {
		log.Println("ERROR embed card missing html")
		return ""
	}

	return html.(string)
}

func (p *post) populate(gi *ghostInfo, gth *GhostToHugo) {
	p.Published = gth.parseTime(p.PublishedAt)
	p.Created = gth.parseTime(p.CreatedAt)
	p.IsDraft = p.Status == "draft"
	p.IsPage = parseBool(p.Page)

	for _, user := range gi.users {
		if bytes.Equal(user.ID, p.AuthorID) {
			p.Author = user.Name
			break
		}
	}

	for _, pt := range gi.posttags {
		if bytes.Equal(pt.PostID, p.ID) {
			for _, t := range gi.tags {
				if bytes.Equal(t.ID, pt.TagID) {
					p.Tags = append(p.Tags, strings.TrimPrefix(t.Name, "#"))
					break
				}
			}
		}
	}
}

func (p post) mobiledocMarkdown() string {
	if p.MobileDoc == "" {
		return ""
	}

	r := strings.NewReader(p.MobileDoc)
	var buf bytes.Buffer

	md := mobiledoc.NewMobiledoc(r).
		WithAtom("soft-break", atomSoftReturn).
		WithAtom("soft-return", atomSoftReturn).
		WithCard("card-markdown", cardMarkdown).
		WithCard("markdown", cardMarkdown).
		WithCard("hr", cardHR).
		WithCard("image", cardImage).
		WithCard("code", cardCode).
		WithCard("embed", cardEmbed).
		WithCard("gallery", cardGallery).
		WithCard("html", cardHTML).
		WithCard("bookmark", cardBookmark)

	err := md.Render(&buf)
	if err != nil {
		log.Printf("ERROR rendering post %s (%v)", p.ID, err)
		return ""
	}

	return buf.String()
}

func (p post) path(site *hugolib.Site) string {
	if p.IsPage {
		return site.PathSpec.AbsPathify(
			path.Join(viper.GetString("contentDir"), p.Slug+".md"))
	}

	return site.PathSpec.AbsPathify(
		path.Join(viper.GetString("contentDir"), "post", p.Slug+".md"))
}

func stripContentFolder(originalString string) string {
	return strings.TrimPrefix(originalString, "/content")
}

func (p post) metadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	switch p.IsDraft {
	case true:
		metadata["date"] = p.Created
	case false:
		metadata["date"] = p.Published
	}
	metadata["title"] = p.Title
	metadata["draft"] = p.IsDraft
	metadata["slug"] = p.Slug
	metadata["description"] = p.MetaDescription
	if p.Image != "" {
		metadata["image"] = stripContentFolder(p.Image)
	} else if p.FeaturedImage != "" {
		metadata["image"] = stripContentFolder(p.FeaturedImage)
	}
	if len(p.Tags) > 0 {
		metadata["tags"] = p.Tags
		metadata["categories"] = p.Tags
	}
	if p.Author != "" {
		metadata["author"] = p.Author
	}

	return metadata
}
