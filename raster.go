package escpos

import (
	"log"
)

const (
	gs8lMaxY = 1662
)

//intLowHigh Generate multiple bytes for a number: In lower and higher parts,
// or more parts as needed.
// :param inp_number: Input number// :param out_bytes:
// The number of bytes to output (1 - 4).
// Function made based on mike42 python-escpos package
func intLowHigh(inpNumber int, outBytes int) []byte {

	maxInput := (256 << (uint((outBytes * 8)) - 1))

	if outBytes < 1 || outBytes > 4 {
		log.Println("Can only output 1-4 bytes")
	}
	if inpNumber < 0 || inpNumber > maxInput {
		log.Println("Number too large. Can only output up to " + string(maxInput) + " in" + string(outBytes) + "byes")
	}
	var outp []byte
	for i := 0; i < outBytes; i++ {
		inpNumberByte := byte(inpNumber % 256)
		outp = append(outp, inpNumberByte)
		inpNumber = inpNumber / 256
	}
	return outp
}

// Raster writes a rasterized version of a black and white image to the printer
// with the specified width, height, and lineWidth bytes per line.
func (p *Printer) Raster(width, height, lineWidth int, imgBw []byte, printingType string) {

	if printingType == "bitImage" {
		densityByte := byte(0)
		header := []byte{0x1D, 0x76, 0x30}
		header = append(header, densityByte)
		width = (width + 7) >> 3
		header = append(header, intLowHigh(width, 2)...)
		header = append(header, intLowHigh(height, 2)...)

		fullImage := append(header, imgBw...)

		p.Write(fullImage)

	} else if printingType == "graphics" {
		for l := 0; l < height; {
			lines := gs8lMaxY
			if lines > height-l {
				lines = height - l
			}

			f112P := 10 + lines*lineWidth

			p.Write([]byte{
				0x1d, 0x38, 0x4c, // GS 8 L, Store the graphics data in the print buffer -- (raster format), p. 252
				byte(f112P), byte(f112P >> 8), byte(f112P >> 16), byte(f112P >> 24), // p1 p2 p3 p4
				0x30, 0x70, 0x30, // function 112
				0x01, 0x01, // bx, by -- zoom
				0x31,                          // c -- single-color printing model
				byte(width), byte(width >> 8), // xl, xh -- number of dots in the horizontal direction
				byte(lines), byte(lines >> 8), // yl, yh -- number of dots in the vertical direction
			})

			// write line
			p.Write(imgBw[l*lineWidth : (l+lines)*lineWidth])

			// flush
			//
			// GS ( L, Print the graphics data in the print buffer,
			//   p. 241 Moves print position to the left side of the
			//   print area after printing of graphics data is
			//   completed
			p.Write([]byte{
				0x1d, 0x28, 0x4c, 0x02, 0x00, 0x30,
				0x32, //  Fn 50
			})

			l += lines
		}
	}
}
