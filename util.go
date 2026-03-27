package escpos

import (
	"encoding/xml"
	"errors"
)

var (
	// ErrBodyElementEmpty is the body element empty error.
	ErrBodyElementEmpty = errors.New("Body element empty")
)

// Node represents a parsed XML node with name, attributes and content
type Node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []Node     `xml:",any"`
}

// Name returns the local name of the XML node
func (n *Node) Name() string {
	return n.XMLName.Local
}

// Attributes returns the attributes as a map
func (n *Node) Attributes() map[string]string {
	attrs := make(map[string]string)
	for _, attr := range n.Attrs {
		attrs[attr.Name.Local] = attr.Value
	}
	return attrs
}

// SOAP envelope structure for parsing
type Envelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    Body     `xml:"Body"`
}

type Body struct {
	XMLName xml.Name `xml:"Body"`
	Content []byte   `xml:",innerxml"`
}

// getBodyChildren returns the child nodes contained in the Body element in a XML document.
func getBodyChildren(data []byte) ([]Node, error) {
	var envelope Envelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}

	if len(envelope.Body.Content) == 0 {
		return nil, ErrBodyElementEmpty
	}

	// Parse the body content as a document fragment with multiple root elements
	bodyContent := "<root>" + string(envelope.Body.Content) + "</root>"

	var wrapper struct {
		XMLName xml.Name `xml:"root"`
		Nodes   []Node   `xml:",any"`
	}

	if err := xml.Unmarshal([]byte(bodyContent), &wrapper); err != nil {
		return nil, err
	}

	if len(wrapper.Nodes) == 0 {
		return nil, ErrBodyElementEmpty
	}

	return wrapper.Nodes, nil
}
