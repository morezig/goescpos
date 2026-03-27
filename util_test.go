package escpos

import (
	"encoding/xml"
	"reflect"
	"testing"
)

// Test data based on real SOAP requests used in pos-proxy
var testSOAPRequests = []struct {
	name     string
	xmlData  string
	expected []Node
}{
	{
		name: "Simple text print request",
		xmlData: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center">Hello World</text>
    <cut type="feed"/>
  </s:Body>
</s:Envelope>`,
		expected: []Node{
			{
				XMLName: xml.Name{Local: "text"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "align"}, Value: "center"}},
				Content: "Hello World",
			},
			{
				XMLName: xml.Name{Local: "cut"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "type"}, Value: "feed"}},
				Content: "",
			},
		},
	},
	{
		name: "Image print request",
		xmlData: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <image width="256" height="256" color="color_1" mode="mono">iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==</image>
    <feed line="2"/>
  </s:Body>
</s:Envelope>`,
		expected: []Node{
			{
				XMLName: xml.Name{Local: "image"},
				Attrs: []xml.Attr{
					{Name: xml.Name{Local: "width"}, Value: "256"},
					{Name: xml.Name{Local: "height"}, Value: "256"},
					{Name: xml.Name{Local: "color"}, Value: "color_1"},
					{Name: xml.Name{Local: "mode"}, Value: "mono"},
				},
				Content: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
			},
			{
				XMLName: xml.Name{Local: "feed"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "line"}, Value: "2"}},
				Content: "",
			},
		},
	},
	{
		name: "Complex receipt with multiple elements",
		xmlData: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center" em="true">RECEIPT</text>
    <feed line="1"/>
    <text align="left">Item 1: $10.00</text>
    <text align="left">Item 2: $15.00</text>
    <feed line="1"/>
    <text align="right" dh="true">Total: $25.00</text>
    <feed line="2"/>
    <cut type="feed"/>
    <pulse/>
  </s:Body>
</s:Envelope>`,
		expected: []Node{
			{
				XMLName: xml.Name{Local: "text"},
				Attrs: []xml.Attr{
					{Name: xml.Name{Local: "align"}, Value: "center"},
					{Name: xml.Name{Local: "em"}, Value: "true"},
				},
				Content: "RECEIPT",
			},
			{
				XMLName: xml.Name{Local: "feed"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "line"}, Value: "1"}},
				Content: "",
			},
			{
				XMLName: xml.Name{Local: "text"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "align"}, Value: "left"}},
				Content: "Item 1: $10.00",
			},
			{
				XMLName: xml.Name{Local: "text"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "align"}, Value: "left"}},
				Content: "Item 2: $15.00",
			},
			{
				XMLName: xml.Name{Local: "feed"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "line"}, Value: "1"}},
				Content: "",
			},
			{
				XMLName: xml.Name{Local: "text"},
				Attrs: []xml.Attr{
					{Name: xml.Name{Local: "align"}, Value: "right"},
					{Name: xml.Name{Local: "dh"}, Value: "true"},
				},
				Content: "Total: $25.00",
			},
			{
				XMLName: xml.Name{Local: "feed"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "line"}, Value: "2"}},
				Content: "",
			},
			{
				XMLName: xml.Name{Local: "cut"},
				Attrs:   []xml.Attr{{Name: xml.Name{Local: "type"}, Value: "feed"}},
				Content: "",
			},
			{
				XMLName: xml.Name{Local: "pulse"},
				Attrs:   []xml.Attr{},
				Content: "",
			},
		},
	},
	{
		name: "Empty body should return error",
		xmlData: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
  </s:Body>
</s:Envelope>`,
		expected: nil, // Should return ErrBodyElementEmpty
	},
}

// Helper functions for creating XML structures in tests

func TestGetBodyChildren(t *testing.T) {
	for _, tc := range testSOAPRequests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := getBodyChildren([]byte(tc.xmlData))
			
			if tc.expected == nil {
				// Expecting an error for empty body
				if err == nil {
					t.Errorf("Expected error for empty body, got nil")
				}
				if err != ErrBodyElementEmpty {
					t.Errorf("Expected ErrBodyElementEmpty, got %v", err)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if len(nodes) != len(tc.expected) {
				t.Errorf("Expected %d nodes, got %d", len(tc.expected), len(nodes))
				return
			}
			
			for i, node := range nodes {
				expected := tc.expected[i]
				
				// Check node name
				if node.Name() != expected.XMLName.Local {
					t.Errorf("Node %d: expected name %q, got %q", i, expected.XMLName.Local, node.Name())
				}
				
				// Check content
				if node.Content != expected.Content {
					t.Errorf("Node %d: expected content %q, got %q", i, expected.Content, node.Content)
				}
				
				// Check attributes
				attrs := node.Attributes()
				expectedAttrs := make(map[string]string)
				for _, attr := range expected.Attrs {
					expectedAttrs[attr.Name.Local] = attr.Value
				}
				
				if !reflect.DeepEqual(attrs, expectedAttrs) {
					t.Errorf("Node %d: expected attributes %v, got %v", i, expectedAttrs, attrs)
				}
			}
		})
	}
}

func TestNodeMethods(t *testing.T) {
	node := Node{
		XMLName: xml.Name{Local: "text"},
		Attrs: []xml.Attr{
			{Name: xml.Name{Local: "align"}, Value: "center"},
			{Name: xml.Name{Local: "em"}, Value: "true"},
		},
		Content: "Hello World",
	}
	
	// Test Name method
	if node.Name() != "text" {
		t.Errorf("Expected name 'text', got %q", node.Name())
	}
	
	// Test Attributes method
	attrs := node.Attributes()
	expected := map[string]string{
		"align": "center",
		"em":    "true",
	}
	
	if !reflect.DeepEqual(attrs, expected) {
		t.Errorf("Expected attributes %v, got %v", expected, attrs)
	}
}

// Benchmark test to compare performance
func BenchmarkGetBodyChildren(b *testing.B) {
	xmlData := []byte(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center" em="true">RECEIPT</text>
    <feed line="1"/>
    <text align="left">Item 1: $10.00</text>
    <text align="left">Item 2: $15.00</text>
    <feed line="1"/>
    <text align="right" dh="true">Total: $25.00</text>
    <feed line="2"/>
    <cut type="feed"/>
    <pulse/>
  </s:Body>
</s:Envelope>`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getBodyChildren(xmlData)
		if err != nil {
			b.Fatal(err)
		}
	}
}