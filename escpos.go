package escpos

import (
	"bufio"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/freetype"
	"golang.org/x/image/font"

	"github.com/cloudinn/escpos/raster"
)

var (
	dpi        = flag.Float64("dpi", 50, "screen resolution in Dots Per Inch")
	fontfile   = flag.String("fontfile", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", "filename of the ttf font")
	hinting    = flag.String("hinting", "none", "none | full")
	size       = flag.Float64("size", 30, "font size in points")
	spacing    = flag.Float64("spacing", 1.5, "line spacing (e.g. 2 means double spaced)")
	wonb       = flag.Bool("whiteonblack", true, "white text on a black background")
	imageHight = flag.Int("imagehight", 38, "define image hight to be printed")
)

// Printer wraps sending ESC-POS commands to a io.Writer.
type Printer struct {
	// destination
	w io.Writer

	// font metrics
	width, height byte

	// state toggles ESC[char]
	underline  byte
	emphasize  byte
	upsidedown byte
	rotate     byte

	// state toggles GS[char]
	reverse, smooth byte

	sync.Mutex
}

// NewPrinter creates a new printer using the specified writer.
func NewPrinter(w io.Writer /*, opts ...PrinterOption*/) (*Printer, error) {
	if w == nil {
		return nil, errors.New("must supply valid writer")
	}

	p := &Printer{
		w:      w,
		width:  1,
		height: 1,
	}

	return p, nil
}

// Reset resets the printer state.
func (p *Printer) Reset() {
	p.width = 1
	p.height = 1

	p.underline = 0
	p.emphasize = 0
	p.upsidedown = 0
	p.rotate = 0

	p.reverse = 0
	p.smooth = 0
}

// Write writes buf to printer.
func (p *Printer) Write(buf []byte) (int, error) {
	return p.w.Write(buf)
}

// WriteString writes a string to the printer.
func (p *Printer) WriteString(s string) (int, error) {
	p.PrintTextImage(s)
	return p.w.Write([]byte(""))
}

// Init resets the state of the printer, and writes the initialize code.
func (p *Printer) Init() {
	p.Reset()
	p.w.Write([]byte("\x1B@"))
}

// End terminates the printer session.
func (p *Printer) End() {
	p.w.Write([]byte("\xFA"))

}

// Cut writes the cut code to the printer.
func (p *Printer) Cut() {
	p.w.Write([]byte("\x1DVA0"))
}

// Cash writes the cash code to the printer.
func (p *Printer) Cash() {
	p.w.Write([]byte("\x1B\x70\x00\x0A\xFF"))
}

// Linefeed writes a line end to the printer.
func (p *Printer) Linefeed() {
	p.w.Write([]byte("\n"))
}

// FormfeedN writes N formfeeds to the printer.
func (p *Printer) FormfeedN(n int) {
	p.w.Write([]byte(fmt.Sprintf("\x1Bd%c", n)))
}

// Formfeed writes 1 formfeed to the printer.
func (p *Printer) Formfeed() {
	p.FormfeedN(1)
}

// SetFont sets the font on the printer.
func (p *Printer) SetFont(font string) {
	f := 0

	switch font {
	case "A":
		f = 0
	case "B":
		f = 1
	case "C":
		f = 2
	default:
		log.Println("Invalid font: '%s', defaulting to 'A'", font)
		f = 0
	}

	p.w.Write([]byte(fmt.Sprintf("\x1BM%c", f)))

}

// SendFontSize sends the font size command to the printer.
func (p *Printer) SendFontSize() {
	p.w.Write([]byte(fmt.Sprintf("\x1D!%c", ((p.width-1)<<4)|(p.height-1))))

}

// SetFontSize sets the font size state and sends the command to the printer.
func (p *Printer) SetFontSize(width, height byte) {
	if width > 0 && height > 0 && width <= 8 && height <= 8 {
		p.width, p.height = width, height
		p.SendFontSize()
	} else {
		log.Println("Invalid font size passed: %d x %d", width, height)
	}
}

// SendUnderline sends the underline command to the printer.
func (p *Printer) SendUnderline() {
	p.w.Write([]byte(fmt.Sprintf("\x1B-%c", p.underline)))
}

// SendEmphasize sends the emphasize / doublestrike command to the printer.
func (p *Printer) SendEmphasize() {
	p.w.Write([]byte(fmt.Sprintf("\x1BG%c", p.emphasize)))

}

// SendUpsidedown sends the upsidedown command to the printer.
func (p *Printer) SendUpsidedown() {
	p.w.Write([]byte(fmt.Sprintf("\x1B{%c", p.upsidedown)))
}

// SendRotate sends the rotate command to the printer.
func (p *Printer) SendRotate() {
	p.w.Write([]byte(fmt.Sprintf("\x1BR%c", p.rotate)))
}

// SendReverse sends the reverse command to the printer.
func (p *Printer) SendReverse() {
	p.w.Write([]byte(fmt.Sprintf("\x1DB%c", p.reverse)))
}

// SendSmooth sends the smooth command to the printer.
func (p *Printer) SendSmooth() {
	p.w.Write([]byte(fmt.Sprintf("\x1Db%c", p.smooth)))

}

// SendMoveX sends the move x command to the printer.
func (p *Printer) SendMoveX(x uint16) {
	p.Write([]byte{0x1b, 0x24, byte(x % 256), byte(x / 256)})
}

// SendMoveY sends the move y command to the printer.
func (p *Printer) SendMoveY(y uint16) {
	p.Write([]byte{0x1d, 0x24, byte(y % 256), byte(y / 256)})
}

// SetUnderline sets the underline state and sends it to the printer.
func (p *Printer) SetUnderline(v byte) {
	p.underline = v
	p.SendUnderline()
}

// SetEmphasize sets the emphasize state and sends it to the printer.
func (p *Printer) SetEmphasize(u byte) {
	p.emphasize = u
	p.SendEmphasize()
}

// SetUpsidedown sets the upsidedown state and sends it to the printer.
func (p *Printer) SetUpsidedown(v byte) {
	p.upsidedown = v
	p.SendUpsidedown()
}

// SetRotate sets the rotate state and sends it to the printer.
func (p *Printer) SetRotate(v byte) {
	p.rotate = v
	p.SendRotate()
}

// SetReverse sets the reverse state and sends it to the printer.
func (p *Printer) SetReverse(v byte) {
	p.reverse = v
	p.SendReverse()
}

// SetSmooth sets the smooth state and sends it to the printer.
func (p *Printer) SetSmooth(v byte) {
	p.smooth = v
	p.SendSmooth()
}

// Pulse sends the pulse (open drawer) code to the printer.
func (p *Printer) Pulse() {
	// with t=2 -- meaning 2*2msec
	p.w.Write([]byte("\x1Bp\x02"))
}

// SetAlign sets the alignment state and sends it to the printer.
func (p *Printer) SetAlign(align string) {
	a := 0
	switch align {
	case "left":
		a = 0
	case "center":
		a = 1
	case "right":
		a = 2
	default:
		log.Println("Invalid alignment: %s", align)
	}
	p.w.Write([]byte(fmt.Sprintf("\x1Ba%c", a)))

}

// SetLang sets the language state and sends it to the printer.
func (p *Printer) SetLang(lang string) {
	l := 0

	switch lang {
	case "en":
		l = 0
	case "fr":
		l = 1
	case "de":
		l = 2
	case "uk":
		l = 3
	case "da":
		l = 4
	case "sv":
		l = 5
	case "it":
		l = 6
	case "es":
		l = 7
	case "ja":
		l = 8
	case "no":
		l = 9
	default:
		log.Println("Invalid language: %s", lang)
	}

	p.w.Write([]byte(fmt.Sprintf("\x1BR%c", l)))

}

// Text sends a block of text to the printer using the formatting parameters in params.
func (p *Printer) Text(params map[string]string, text string) error {
	// send alignment to printer
	if align, ok := params["align"]; ok {
		p.SetAlign(align)
	}

	// set lang
	if lang, ok := params["lang"]; ok {
		p.SetLang(lang)
	}

	// set smooth
	if smooth, ok := params["smooth"]; ok && (smooth == "true" || smooth == "1") {
		p.SetSmooth(1)
	}

	// set emphasize
	if em, ok := params["em"]; ok && (em == "true" || em == "1") {
		p.SetEmphasize(1)
	}

	// set underline
	if ul, ok := params["ul"]; ok && (ul == "true" || ul == "1") {
		p.SetUnderline(1)
	}

	// set reverse
	if reverse, ok := params["reverse"]; ok && (reverse == "true" || reverse == "1") {
		p.SetReverse(1)
	}

	// set rotate
	if rotate, ok := params["rotate"]; ok && (rotate == "true" || rotate == "1") {
		p.SetRotate(1)
	}

	// set font
	if font, ok := params["font"]; ok {
		p.SetFont(strings.ToUpper(font[5:6]))
	}

	// do dw (double font width)
	if dw, ok := params["dw"]; ok && (dw == "true" || dw == "1") {
		p.SetFontSize(2, p.height)
	}

	// do dh (double font height)
	if dh, ok := params["dh"]; ok && (dh == "true" || dh == "1") {
		p.SetFontSize(p.width, 2)
	}

	// do font width
	if width, ok := params["width"]; ok {
		if i, err := strconv.Atoi(width); err == nil {
			p.SetFontSize(byte(i), p.height)
		} else {
			// log.Println("Invalid font width: %s", width)
			return err
		}
	}

	// do font height
	if height, ok := params["height"]; ok {
		if i, err := strconv.Atoi(height); err == nil {
			p.SetFontSize(p.width, byte(i))
		} else {
			// log.Fatalf("Invalid font height: %s", height)
			return err
		}
	}

	// do y positioning
	if x, ok := params["x"]; ok {
		if i, err := strconv.Atoi(x); err == nil {
			p.SendMoveX(uint16(i))
		} else {
			// log.Fatalf("Invalid x param %s", x)
			return err
		}
	}

	// do y positioning
	if y, ok := params["y"]; ok {
		if i, err := strconv.Atoi(y); err == nil {
			p.SendMoveY(uint16(i))
		} else {
			// log.Fatalf("Invalid y param %s", y)
			return err
		}
	}

	// do text replace, then write data
	if len(text) > 0 {
		p.WriteString(textReplacer.Replace(text))
	}

	return nil
}

// Feed feeds the printer, applying the supplied params as necessary.
func (p *Printer) Feed(params map[string]string) error {
	// handle lines (form feed X lines)
	if l, ok := params["line"]; ok {
		if i, err := strconv.Atoi(l); err == nil {
			p.FormfeedN(i)
		} else {
			// log.Fatalf("Invalid line number %s", l)
			return err
		}
	}

	// handle units (dots)
	if u, ok := params["unit"]; ok {
		if i, err := strconv.Atoi(u); err == nil {
			p.SendMoveY(uint16(i))
		} else {
			// log.Fatalf("Invalid unit number %s", u)
			return err
		}
	}

	// send linefeed
	p.Linefeed()

	// reset variables
	p.Reset()

	// reset printer
	p.SendEmphasize()
	p.SendRotate()
	p.SendSmooth()
	p.SendReverse()
	p.SendUnderline()
	p.SendUpsidedown()
	p.SendFontSize()
	p.SendUnderline()

	return nil
}

// FeedAndCut feeds the printer using the supplied params and then sends a cut
// command.
func (p *Printer) FeedAndCut(params map[string]string) {
	if t, ok := params["type"]; ok && t == "feed" {
		p.Formfeed()
	}

	p.Cut()
}

// Barcode sends a barcode to the printer.
func (p *Printer) Barcode(barcode string, format int) {
	code := ""
	switch format {
	case 0:
		code = "\x00"
	case 1:
		code = "\x01"
	case 2:
		code = "\x02"
	case 3:
		code = "\x03"
	case 4:
		code = "\x04"
	case 73:
		code = "\x49"
	}

	// reset settings
	p.Reset()

	// set align
	p.SetAlign("center")

	// write barcode
	if format > 69 {
		p.w.Write([]byte(fmt.Sprintf("\x1dk"+code+"%v%v", len(barcode), barcode)))

	} else if format < 69 {
		p.w.Write([]byte(fmt.Sprintf("\x1dk"+code+"%v\x00", barcode)))
	}
	p.w.Write([]byte(barcode))

}

// gSendsend graphics headers.
func (p *Printer) gSend(m byte, fn byte, data []byte) {
	l := len(data) + 2

	p.w.Write([]byte("\x1b(L"))
	p.Write([]byte{byte(l % 256), byte(l / 256), m, fn})
	p.w.Write([]byte(data))

}

// Image writes an image using the supplied params.
func (p *Printer) Image(params map[string]string, data string) error {
	// send alignment to printer
	if align, ok := params["align"]; ok {
		p.SetAlign(align)
	}

	// get width
	wstr, ok := params["width"]
	if !ok {
		log.Println("No width specified on image")
	}

	// get height
	hstr, ok := params["height"]
	if !ok {
		log.Println("No height specified on image")
	}

	// convert width
	width, err := strconv.Atoi(wstr)
	if err != nil {
		// log.Println("Invalid image width %s", wstr)
		return err
	}

	// convert height
	height, err := strconv.Atoi(hstr)
	if err != nil {
		// log.Fatalf("Invalid image height %s", hstr)
		return err
	}

	// decode data frome b64 string
	dec, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		// log.Fatal(err)
		return err
	}

	log.Printf("Image len:%d w: %d h: %d\n", len(dec), width, height)

	// $imgHeader = self::dataHeader(array($img -> getWidth(), $img -> getHeight()), true);
	// $tone = '0';
	// $colors = '1';
	// $xm = (($size & self::IMG_DOUBLE_WIDTH) == self::IMG_DOUBLE_WIDTH) ? chr(2) : chr(1);
	// $ym = (($size & self::IMG_DOUBLE_HEIGHT) == self::IMG_DOUBLE_HEIGHT) ? chr(2) : chr(1);
	//
	// $header = $tone . $xm . $ym . $colors . $imgHeader;
	// $this -> graphicsSendData('0', 'p', $header . $img -> toRasterFormat());
	// $this -> graphicsSendData('0', '2');

	header := []byte{
		byte('0'), 0x01, 0x01, byte('1'),
	}

	a := append(header, dec...)

	p.gSend(byte('0'), byte('p'), a)
	p.gSend(byte('0'), byte('2'), []byte{})

	return nil
}

// WriteNode writes a node of type name with the supplied params and data to
// the printer.
func (p *Printer) WriteNode(name string, params map[string]string, data string) {
	cstr := ""
	if data != "" {
		str := data
		if len(data) > 40 {
			str = fmt.Sprintf("%s ...", data[0:40])
		}
		cstr = fmt.Sprintf(" => '%s'", str)
	}
	log.Printf("Write: %s => %+v%s\n", name, params, cstr)

	switch name {
	case "text":
		p.Text(params, data)

	case "feed":
		p.Feed(params)

	case "cut":
		p.FeedAndCut(params)

	case "pulse":
		p.Pulse()

	case "image":
		p.Image(params, data)
	}
}

// textReplacer is a simple text replacer for the only valid XML encoded
// entities for escpos printers.
var textReplacer = strings.NewReplacer(
	// horizontal tab
	"&#9;", "\x09",
	"&#x9;", "\x09",

	// linefeed
	"&#10;", "\n",
	"&#xA;", "\n",

	// xml entities
	"&apos;", "'",
	"&quot;", `"`,
	"&gt;", ">",
	"&lt;", "<",

	// &amp; (ampersand) must be last to avoid double decoding
	"&amp;", "&",
)

// PrintImage Print Image
func (p *Printer) PrintImage(imgPath string) error {
	imgFile, err := os.Open(imgPath)
	if err != nil {
		// log.Fatal(err)
		return err
	}

	img, imgFormat, err := image.Decode(imgFile)
	imgFile.Close()
	if err != nil {
		// log.Fatal(err)
		return err
	}
	log.Print("Loaded image, format: ", imgFormat)

	rasterConv := &raster.Converter{
		MaxWidth:  512,
		Threshold: 0.5,
	}
	p.SetAlign("center")
	rasterConv.Print(img, p)
	return nil
}

//SetWhiteOnBlack sets the background for the image to white for true or black for false
func (p *Printer) SetWhiteOnBlack(wonbVal bool) {
	*wonb = wonbVal
}

//SetFontSizePoint sets font size in points for some selected font
func (p *Printer) SetFontSizePoints(fontSize float64) {
	*size = fontSize
}

//SetDPI sets resolution in dots per inch for the image
func (p *Printer) SetDPI(resolution float64) {
	*dpi = resolution
}

//SetFontFile to choose a certien font to print the image with
func (p *Printer) SetFontFile(filepath string) {
	*fontfile = filepath
}

//SetHinting sets hinting
func (p *Printer) SetHinting(hintingVal string) {
	*hinting = hintingVal
}

//SetSpacing set spacing between lines in image
func (p *Printer) SetSpacing(spacingVal float64) {
	*spacing = spacingVal
}

func (p *Printer) SetImageHight(hight int) {
	*imageHight = hight
}

//PrintTextImage takes a string convert it to an image and print it
func (p *Printer) PrintTextImage(text string) error {
	// flag.Parse()
	// Read the font data.
	fontBytes, err := ioutil.ReadFile(*fontfile)
	if err != nil {
		return err
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return err
	}

	// Initialize the context.
	fg, bg := image.Black, image.White
	ruler := color.RGBA{0xdd, 0xdd, 0xdd, 0xff}
	if *wonb {
		fg, bg = image.White, image.Black
		ruler = color.RGBA{0x22, 0x22, 0x22, 0xff}
	}
	rgba := image.NewRGBA(image.Rect(0, 0, 760, *imageHight))
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)
	c := freetype.NewContext()
	c.SetDPI(*dpi)
	c.SetFont(f)
	c.SetFontSize(*size)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(fg)
	switch *hinting {
	default:
		c.SetHinting(font.HintingNone)
	case "full":
		c.SetHinting(font.HintingFull)
	}

	// Draw the guidelines.
	for i := 0; i < 200; i++ {
		rgba.Set(10, 10+i, ruler)
		rgba.Set(10+i, 10, ruler)
	}

	// Draw the text.
	pt := freetype.Pt(10, 10+int(c.PointToFixed(*size)>>6))
	_, err = c.DrawString(text, pt)
	if err != nil {
		return err
	}
	pt.Y += c.PointToFixed(*size * *spacing)

	// Save that RGBA image to disk.
	outFile, err := os.Create("/var/tmp/posTextImage.png")
	if err != nil {
		// os.Exit(1)
		return err
	}

	defer outFile.Close()
	b := bufio.NewWriter(outFile)
	err = png.Encode(b, rgba)
	if err != nil {
		// os.Exit(1)
		return err
	}
	err = b.Flush()
	if err != nil {
		// os.Exit(1)
		return err
	}

	p.PrintImage(outFile.Name())

	return nil
}

// TextToRaster takes a string, font size, boolean value if true will print text black background white
// if false will print text white background black
// return slice bytes of raster image with width and height
func (p *Printer) TextToRaster(text string, fontSize float64, wb bool) (data []byte, width int, height int, err error) {
	fontBytes, err := ioutil.ReadFile(*fontfile)
	if err != nil {
		return nil, 0, 0, err
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, 0, 0, err
	}

	// Initialize the context.
	fg, bg := image.Black, image.White
	ruler := color.RGBA{0xdd, 0xdd, 0xdd, 0xff}
	if wb {
		fg, bg = image.White, image.Black
		ruler = color.RGBA{0x22, 0x22, 0x22, 0xff}
	}
	rgba := image.NewRGBA(image.Rect(0, 0, 760, *imageHight))
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)
	c := freetype.NewContext()
	c.SetDPI(*dpi)
	c.SetFont(f)
	c.SetFontSize(fontSize)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(fg)
	switch *hinting {
	default:
		c.SetHinting(font.HintingNone)
	case "full":
		c.SetHinting(font.HintingFull)
	}

	// Draw the guidelines.
	for i := 0; i < 200; i++ {
		rgba.Set(10, 10+i, ruler)
		rgba.Set(10+i, 10, ruler)
	}

	// Draw the text.
	pt := freetype.Pt(10, 10+int(c.PointToFixed(fontSize)>>6))
	_, err = c.DrawString(text, pt)
	if err != nil {
		return nil, 0, 0, err
	}
	pt.Y += c.PointToFixed(fontSize * *spacing)

	rasterConv := &raster.Converter{
		MaxWidth:  512,
		Threshold: 0.5,
	}

	height = rgba.Bounds().Size().Y

	data, width, _ = rasterConv.ToRaster(rgba)

	return data, width, height, nil
}
