package mime

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const jscode = `#!/bin/node
function main(){
}
`

func TestJs(t *testing.T) {
	m := DetectAny([]byte(jscode))
	fmt.Fprintf(os.Stderr, "%v\n", m.String())
}

const h5 = `<!DOCTYPE html>
<html>
  <head><!--[if lt IE 9]><script language="javascript" type="text/javascript" src="//html5shim.googlecode.com/svn/trunk/html5.js"></script><![endif]-->
     </style>
    <link rel="stylesheet" href="css/animation.css"><!--[if IE 7]><link rel="stylesheet" href="css/" + font.fontname + "-ie7.css"><![endif]-->
    <script>
    </script>
  </head>
  <body>
    <div class="container footer"></div>
  </body>
</html>

`

func TestH5(t *testing.T) {
	m := DetectAny([]byte(h5))
	fmt.Fprintf(os.Stderr, "%v\n", m.String())
}

const svgblock = `<?xml version="1.0" encoding="utf-8"?>

<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.0//EN" "http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd">
<!-- Uploaded to: SVG Repo, www.svgrepo.com, Generator: SVG Repo Mixer Tools -->
<svg version="1.0" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" 
	 width="800px" height="800px" viewBox="0 0 64 64" enable-background="new 0 0 64 64" xml:space="preserve">
<g>
	<path fill="#394240" d="M48,5c-4.418,0-8.418,1.793-11.312,4.688L32,14.344l-4.688-4.656C24.418,6.793,20.418,5,16,5
		C7.164,5,0,12.164,0,21c0,4.418,2.852,8.543,5.75,11.438l23.422,23.426c1.562,1.562,4.094,1.562,5.656,0L58.188,32.5
		C61.086,29.605,64,25.418,64,21C64,12.164,56.836,5,48,5z M32,47.375L11.375,26.75C9.926,25.305,8,23.211,8,21c0-4.418,3.582-8,8-8
		c2.211,0,4.211,0.895,5.656,2.344l7.516,7.484c1.562,1.562,4.094,1.562,5.656,0l7.516-7.484C43.789,13.895,45.789,13,48,13
		c4.418,0,8,3.582,8,8c0,2.211-1.926,4.305-3.375,5.75L32,47.375z"/>
	<path fill="#F76D57" d="M32,47.375L11.375,26.75C9.926,25.305,8,23.211,8,21c0-4.418,3.582-8,8-8c2.211,0,4.211,0.895,5.656,2.344
		l7.516,7.484c1.562,1.562,4.094,1.562,5.656,0l7.516-7.484C43.789,13.895,45.789,13,48,13c4.418,0,8,3.582,8,8
		c0,2.211-1.926,4.305-3.375,5.75L32,47.375z"/>
</g>
</svg>`

func TestSVG(t *testing.T) {
	now := time.Now()
	m := DetectAny([]byte(svgblock))
	fmt.Fprintf(os.Stderr, "%v spent: %v\n", m.String(), time.Since(now))
}

const svgblockNoComment = `<?xml version="1.0" encoding="utf-8"?>
<svg version="1.0" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" 
	 width="800px" height="800px" viewBox="0 0 64 64" enable-background="new 0 0 64 64" xml:space="preserve">
<g>
	<path fill="#394240" d="M48,5c-4.418,0-8.418,1.793-11.312,4.688L32,14.344l-4.688-4.656C24.418,6.793,20.418,5,16,5
		C7.164,5,0,12.164,0,21c0,4.418,2.852,8.543,5.75,11.438l23.422,23.426c1.562,1.562,4.094,1.562,5.656,0L58.188,32.5
		C61.086,29.605,64,25.418,64,21C64,12.164,56.836,5,48,5z M32,47.375L11.375,26.75C9.926,25.305,8,23.211,8,21c0-4.418,3.582-8,8-8
		c2.211,0,4.211,0.895,5.656,2.344l7.516,7.484c1.562,1.562,4.094,1.562,5.656,0l7.516-7.484C43.789,13.895,45.789,13,48,13
		c4.418,0,8,3.582,8,8c0,2.211-1.926,4.305-3.375,5.75L32,47.375z"/>
	<path fill="#F76D57" d="M32,47.375L11.375,26.75C9.926,25.305,8,23.211,8,21c0-4.418,3.582-8,8-8c2.211,0,4.211,0.895,5.656,2.344
		l7.516,7.484c1.562,1.562,4.094,1.562,5.656,0l7.516-7.484C43.789,13.895,45.789,13,48,13c4.418,0,8,3.582,8,8
		c0,2.211-1.926,4.305-3.375,5.75L32,47.375z"/>
</g>
</svg>`

func TestSVGNoComment(t *testing.T) {
	m := DetectAny([]byte(svgblockNoComment))
	fmt.Fprintf(os.Stderr, "%v\n", m.String())
}

const (
	htmlText = `<!DOCTYPE html>
<html>
<head>
<script>
function myFunction()
{
    alert("你好，我是一个警告框！");
}
</script>
</head>
<body>

<input type="button" onclick="myFunction()" value="显示警告框">

</body>
</html>`
)

func TestHTML2(t *testing.T) {
	m := DetectAny([]byte(htmlText))
	fmt.Fprintf(os.Stderr, "%v\n", m.String())
}

func TestSVG2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "mimetsx"))
	if err != nil {
		return
	}
	m := DetectAny(b)
	fmt.Fprintf(os.Stderr, "%v %s\n", m.String(), http.DetectContentType(b))
	m2 := DetectAny([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multline
	Comment -->
	<svg></svg>`))
	fmt.Fprintf(os.Stderr, "%s\n", m2.String())
}

func TestJsonMIME(t *testing.T) {
	for p := json; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			fmt.Fprintf(os.Stderr, "text: %v\n", json.String())
		}
	}
	m2 := DetectAny([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multline
	Comment -->
	<svg></svg>`))
	for p := m2; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			fmt.Fprintf(os.Stderr, "text: %v\n", m2.String())
		}
	}
}

func TestSVGForEach(t *testing.T) {
	ss := []string{
		"<svg></svg>",
		`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1 Basic//EN"
	"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11-basic.dtd">
	<svg></svg>`,
		`<?xml version="1.0" encoding="UTF-8"?><svg></svg>`,
		"var svgText=`<svg></svg>`",
		`<?xml version='1.0' encoding='UTF-8' standalone='no'?>
		<!-- Created with Fritzing (http://www.fritzing.org/) -->
		<svg width="3.50927in"
			 x="0in"
			 version="1.2"
			 y="0in"
			 xmlns="http://www.w3.org/2000/svg"
			 height="2.81713in"
			 viewBox="0 0 252.667 202.833"
			 baseProfile="tiny"
			 xmlns:svg="http://www.w3.org/2000/svg">`,
	}
	for _, s := range ss {
		m := DetectAny([]byte(s))
		fmt.Fprintf(os.Stderr, "[%s]\n mime: %v\n", s, m.mime)
	}
}
