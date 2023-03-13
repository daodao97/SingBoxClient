package abx

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"io"
	"strconv"

	. "github.com/sagernet/sing/common/abx/internal"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ xml.TokenReader = (*Reader)(nil)

type Reader struct {
	reader     *bytes.Reader
	stringRefs []string
	attrs      []xml.Attr
}

func NewReader(content []byte) (xml.TokenReader, bool) {
	if len(content) < 4 || !bytes.Equal(content[:4], ProtocolMagicVersion0) {
		return nil, false
	}
	return &Reader{reader: bytes.NewReader(content[4:])}, true
}

func (r *Reader) Token() (token xml.Token, err error) {
	event, err := r.reader.ReadByte()
	if err != nil {
		return
	}
	tokenType := event & 0x0f
	eventType := event & 0xf0
	switch tokenType {
	case StartDocument:
		return
	case EndDocument:
		return nil, io.EOF
	case StartTag:
		var name string
		name, err = r.readInternedUTF()
		if err != nil {
			return
		}
		var attrs []xml.Attr
		attrs, err = r.pullAttributes()
		if err != nil {
			return
		}
		return xml.StartElement{Name: xml.Name{Local: name}, Attr: attrs}, nil
	case EndTag:
		var name string
		name, err = r.readInternedUTF()
		if err != nil {
			return
		}
		return xml.EndElement{Name: xml.Name{Local: name}}, nil
	case TEXT:
		var data string
		data, err = r.readUTF()
		if err != nil {
			return
		}
		return xml.CharData(data), nil
	case CDSECT:
		var data string
		data, err = r.readUTF()
		if err != nil {
			return
		}
		return xml.Directive("<![CDATA[" + data + "]]>"), nil
	case ProcessingInstruction:
		_, err = r.readUTF()
		return
	case COMMENT:
		var data string
		data, err = r.readUTF()
		if err != nil {
			return
		}
		return xml.Comment(data), nil
	case DOCDECL:
		_, err = r.readUTF()
		return
	case IgnorableWhitespace:
		_, err = r.readUTF()
		return
	case EntityRef:
		_, err = r.readUTF()
		return
	case ATTRIBUTE:
		return nil, E.New("unexpected attribute")
	}
	return nil, E.New("unknown token type ", tokenType, " with type ", eventType)
}

func (r *Reader) pullAttributes() ([]xml.Attr, error) {
	err := r.pullAttribute()
	if err != nil {
		return nil, err
	}
	attrs := r.attrs
	r.attrs = nil
	return attrs, nil
}

func (r *Reader) pullAttribute() error {
	event, err := r.reader.ReadByte()
	if err != nil {
		return nil
	}
	tokenType := event & 0x0f
	eventType := event & 0xf0
	if tokenType != ATTRIBUTE {
		return r.reader.UnreadByte()
	}
	var name string
	name, err = r.readInternedUTF()
	if err != nil {
		return err
	}
	var value string
	switch eventType {
	case TypeNull:
		value = ""
	case TypeBooleanTrue:
		value = "true"
	case TypeBooleanFalse:
		value = "false"
	case TypeString:
		value, err = r.readUTF()
		if err != nil {
			return err
		}
	case TypeStringInterned:
		value, err = r.readInternedUTF()
		if err != nil {
			return err
		}
	case TypeBytesHex:
		var data []byte
		data, err = r.readBytes()
		if err != nil {
			return err
		}
		value = hex.EncodeToString(data)
	case TypeBytesBase64:
		var data []byte
		data, err = r.readBytes()
		if err != nil {
			return err
		}
		value = base64.StdEncoding.EncodeToString(data)
	case TypeInt:
		var data int32
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = strconv.FormatInt(int64(data), 10)
	case TypeIntHex:
		var data int32
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = "0x" + strconv.FormatInt(int64(data), 16)
	case TypeLong:
		var data int64
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = strconv.FormatInt(data, 10)
	case TypeLongHex:
		var data int64
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = "0x" + strconv.FormatInt(data, 16)
	case TypeFloat:
		var data float32
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = strconv.FormatFloat(float64(data), 'g', -1, 32)
	case TypeDouble:
		var data float64
		err = binary.Read(r.reader, binary.BigEndian, &data)
		if err != nil {
			return err
		}
		value = strconv.FormatFloat(data, 'g', -1, 64)
	default:
		return E.New("unexpected attribute type, ", eventType)
	}
	r.attrs = append(r.attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	return r.pullAttribute()
}

func (r *Reader) readUnsignedShort() (uint16, error) {
	var value uint16
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *Reader) readInternedUTF() (utf string, err error) {
	ref, err := r.readUnsignedShort()
	if err != nil {
		return
	}
	if ref == MaxUnsignedShort {
		utf, err = r.readUTF()
		if err != nil {
			return
		}
		if len(r.stringRefs) < MaxUnsignedShort {
			r.stringRefs = append(r.stringRefs, utf)
		}
		return
	}
	if int(ref) >= len(r.stringRefs) {
		err = E.New("invalid interned reference: ", ref, ", exists: ", len(r.stringRefs))
		return
	}
	utf = r.stringRefs[ref]
	return
}

func (r *Reader) readUTF() (utf string, err error) {
	data, err := r.readBytes()
	if err != nil {
		return
	}
	utf = string(data)
	return
}

func (r *Reader) readBytes() (data []byte, err error) {
	length, err := r.readUnsignedShort()
	if err != nil {
		return
	}
	data = make([]byte, length)
	_, err = io.ReadFull(r.reader, data)
	return
}
