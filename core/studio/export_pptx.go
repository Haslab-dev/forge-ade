package studio

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ExportSlidesToPPTX writes a valid 16:9 OOXML PowerPoint (.pptx) file to destPath
// from the provided slides. If slides is empty and htmlPath is provided, it extracts
// slide content from the HTML file first.
func ExportSlidesToPPTX(deckTitle string, slides []SlideItem, htmlPath string, destPath string) error {
	if len(slides) == 0 && htmlPath != "" {
		if raw, err := os.ReadFile(htmlPath); err == nil {
			slides = extractSlidesFromHTML(string(raw), deckTitle)
		}
	}
	if len(slides) == 0 {
		slides = DemoSlides()
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create pptx: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	addFile := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	// 1. [Content_Types].xml
	var slideOverrides strings.Builder
	for i := range slides {
		slideOverrides.WriteString(fmt.Sprintf(
			`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`,
			i+1,
		))
	}
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		slideOverrides.String() + `</Types>`
	if err := addFile("[Content_Types].xml", contentTypes); err != nil {
		return err
	}

	// 2. _rels/.rels
	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`
	if err := addFile("_rels/.rels", rootRels); err != nil {
		return err
	}

	// 3. ppt/presentation.xml & ppt/_rels/presentation.xml.rels
	var sldIdLst strings.Builder
	var presRels strings.Builder
	presRels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := range slides {
		rID := fmt.Sprintf("rId%d", i+2)
		sldIdLst.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="%s"/>`, 256+i, rID))
		presRels.WriteString(fmt.Sprintf(
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`,
			rID, i+1,
		))
	}
	presRels.WriteString(`</Relationships>`)

	// 16:9 widescreen slide size: 12192000 x 6858000 EMU (13.333" x 7.5")
	presentationXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:sldIdLst>` + sldIdLst.String() + `</p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`
	if err := addFile("ppt/presentation.xml", presentationXML); err != nil {
		return err
	}
	if err := addFile("ppt/_rels/presentation.xml.rels", presRels.String()); err != nil {
		return err
	}

	// 4. Minimal SlideMaster, SlideLayout, and Theme
	slideMasterXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
</p:sldMaster>`
	slideMasterRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
	slideLayoutXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank">
  <p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
</p:sldLayout>`
	slideLayoutRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`
	themeXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="ForgeADE">
  <a:themeElements>
    <a:clrScheme name="ForgeADE">
      <a:dk1><a:srgbClr val="0F172A"/></a:dk1><a:lt1><a:srgbClr val="F8FAFC"/></a:lt1>
      <a:dk2><a:srgbClr val="1E293B"/></a:dk2><a:lt2><a:srgbClr val="E2E8F0"/></a:lt2>
      <a:accent1><a:srgbClr val="6366F1"/></a:accent1><a:accent2><a:srgbClr val="06B6D4"/></a:accent2>
      <a:accent3><a:srgbClr val="10B981"/></a:accent3><a:accent4><a:srgbClr val="F59E0B"/></a:accent4>
      <a:accent5><a:srgbClr val="EC4899"/></a:accent5><a:accent6><a:srgbClr val="8B5CF6"/></a:accent6>
      <a:hlink><a:srgbClr val="38BDF8"/></a:hlink><a:folHlink><a:srgbClr val="A855F7"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="ForgeADE">
      <a:majorFont><a:latin typeface="Helvetica Neue"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>
      <a:minorFont><a:latin typeface="Helvetica Neue"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="ForgeADE">
      <a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>
      <a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>
      <a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
      <a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`

	for path, content := range map[string]string{
		"ppt/slideMasters/slideMaster1.xml":            slideMasterXML,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": slideMasterRels,
		"ppt/slideLayouts/slideLayout1.xml":            slideLayoutXML,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": slideLayoutRels,
		"ppt/theme/theme1.xml":                         themeXML,
	} {
		if err := addFile(path, content); err != nil {
			return err
		}
	}

	// 5. Slides
	slideRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`

	for i, s := range slides {
		xmlStr := buildSlideXML(i+1, len(slides), s)
		if err := addFile(fmt.Sprintf("ppt/slides/slide%d.xml", i+1), xmlStr); err != nil {
			return err
		}
		if err := addFile(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), slideRels); err != nil {
			return err
		}
	}

	return zw.Close()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func buildSlideXML(num, total int, s SlideItem) string {
	title := strings.TrimSpace(s.Title)
	if title == "" {
		title = fmt.Sprintf("Slide %d", num)
	}

	var bodyParas strings.Builder
	if strings.TrimSpace(s.Subtitle) != "" {
		bodyParas.WriteString(fmt.Sprintf(
			`<a:p><a:r><a:rPr lang="en-US" sz="2000" b="0"><a:solidFill><a:srgbClr val="94A3B8"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r></a:p>`,
			xmlEscape(strings.TrimSpace(s.Subtitle)),
		))
	}
	for _, b := range s.Bullets {
		bt := strings.TrimSpace(b)
		if bt == "" {
			continue
		}
		bodyParas.WriteString(fmt.Sprintf(
			`<a:p><a:pPr marL="285750" indent="-171450"><a:buChar char="•"/></a:pPr><a:r><a:rPr lang="en-US" sz="1800"><a:solidFill><a:srgbClr val="E2E8F0"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r></a:p>`,
			xmlEscape(bt),
		))
	}
	if bodyParas.Len() == 0 {
		bodyParas.WriteString(`<a:p><a:endParaRPr lang="en-US"/></a:p>`)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgPr><a:solidFill><a:srgbClr val="0F111A"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="SlideNumber"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="685800" y="457200"/><a:ext cx="2400000" cy="360000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
        <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="1200" b="1"><a:solidFill><a:srgbClr val="818CF8"/></a:solidFill></a:rPr><a:t>%02d / %02d</a:t></a:r></a:p></p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Title"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="685800" y="960000"/><a:ext cx="10820400" cy="1100000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
        <p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="3600" b="1"><a:solidFill><a:srgbClr val="F8FAFC"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r></a:p></p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="4" name="Body"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="685800" y="2200000"/><a:ext cx="10820400" cy="4000000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
        <p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>%s</p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, num, total, xmlEscape(title), bodyParas.String())
}

var (
	htmlSectionRe = regexp.MustCompile(`(?is)<section[^>]*>(.*?)</section>`)
	htmlHeadingRe = regexp.MustCompile(`(?is)<h[1-3][^>]*>(.*?)</h[1-3]>`)
	htmlItemRe    = regexp.MustCompile(`(?is)<(?:li|p)[^>]*>(.*?)</(?:li|p)>`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
)

func stripHTMLTags(s string) string {
	clean := htmlTagRe.ReplaceAllString(s, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.TrimSpace(clean)
}

func extractSlidesFromHTML(htmlStr, fallbackTitle string) []SlideItem {
	matches := htmlSectionRe.FindAllStringSubmatch(htmlStr, -1)
	if len(matches) == 0 {
		// Single slide from document headings/paragraphs
		title := fallbackTitle
		if hm := htmlHeadingRe.FindStringSubmatch(htmlStr); len(hm) > 1 {
			if t := stripHTMLTags(hm[1]); t != "" {
				title = t
			}
		}
		var bullets []string
		for _, im := range htmlItemRe.FindAllStringSubmatch(htmlStr, 8) {
			if t := stripHTMLTags(im[1]); t != "" {
				bullets = append(bullets, t)
			}
		}
		if title != "" || len(bullets) > 0 {
			return []SlideItem{{Title: title, Bullets: bullets}}
		}
		return nil
	}

	var out []SlideItem
	for idx, m := range matches {
		body := m[1]
		title := fmt.Sprintf("Slide %d", idx+1)
		if hm := htmlHeadingRe.FindStringSubmatch(body); len(hm) > 1 {
			if t := stripHTMLTags(hm[1]); t != "" {
				title = t
			}
		}
		var bullets []string
		for _, im := range htmlItemRe.FindAllStringSubmatch(body, 10) {
			if t := stripHTMLTags(im[1]); t != "" && t != title {
				bullets = append(bullets, t)
			}
		}
		out = append(out, SlideItem{Title: title, Bullets: bullets})
	}
	return out
}
