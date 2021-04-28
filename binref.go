package gabbygrove

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/ugorji/go/codec"
	"golang.org/x/crypto/ed25519"

	refs "go.mindeco.de/ssb-refs"
)

type RefType uint

const (
	RefTypeUndefined RefType = iota
	RefTypeFeed
	RefTypeMessage
	RefTypeContent
)

// BinaryRef defines a binary representation for feed, message, and content references
type BinaryRef struct {
	fr *refs.FeedRef
	mr *refs.MessageRef
	cr *ContentRef // payload/content ref
}

// currently all references are 32bytes long
// one additional byte for tagging the type
const binrefSize = 33

func (ref BinaryRef) valid() (RefType, error) {
	i := 0
	var t RefType = RefTypeUndefined
	if ref.fr != nil {
		i++
		t = RefTypeFeed
	}
	if ref.mr != nil {
		i++
		t = RefTypeMessage
	}
	if ref.cr != nil {
		i++
		t = RefTypeContent
	}
	if i > 1 {
		return RefTypeUndefined, fmt.Errorf("more than one ref in binref")
	}
	return t, nil
}

func (ref BinaryRef) Ref() string {
	t, err := ref.valid()
	if err != nil {
		panic(err)
	}
	r, err := ref.GetRef(t)
	if err != nil {
		panic(err)
	}
	return r.Ref()
}

func (ref BinaryRef) MarshalBinary() ([]byte, error) {
	t, err := ref.valid()
	if err != nil {
		return nil, err
	}
	switch t {
	case RefTypeFeed:
		return append([]byte{0x01}, ref.fr.PubKey()...), nil
	case RefTypeMessage:
		hd := make([]byte, 32)
		err := ref.mr.CopyHashTo(hd)
		return append([]byte{0x02}, hd...), err
	case RefTypeContent:
		if ref.cr.algo != RefAlgoContentGabby {
			return nil, errors.Errorf("invalid binary content ref for feed: %s", ref.cr.algo)
		}
		crBytes, err := ref.cr.MarshalBinary()
		return append([]byte{0x03}, crBytes[1:]...), err
	default:
		// TODO: check if nil!?
		return nil, nil
	}
}

func (ref *BinaryRef) UnmarshalBinary(data []byte) error {
	if n := len(data); n != binrefSize {
		return errors.Errorf("binref: invalid len:%d", n)
	}
	switch data[0] {
	case 0x01:
		fr, err := refs.NewFeedRefFromBytes(data[1:], refs.RefAlgoFeedGabby)
		if err != nil {
			return err
		}
		ref.fr = &fr
	case 0x02:
		mr, err := refs.NewMessageRefFromBytes(data[1:], refs.RefAlgoMessageGabby)
		if err != nil {
			return err
		}
		ref.mr = &mr
	case 0x03:
		var newCR ContentRef
		if err := newCR.UnmarshalBinary(append([]byte{0x02}, data[1:]...)); err != nil {
			return err
		}
		if newCR.Algo() != RefAlgoContentGabby {
			return errors.Errorf("unmarshal: invalid binary content ref for feed: %q", newCR.algo)
		}
		ref.cr = &newCR
	default:
		return fmt.Errorf("unmarshal: invalid binref type: %x", data[0])
	}
	return nil
}

func (ref *BinaryRef) Size() int {
	return binrefSize
}

func (ref BinaryRef) MarshalJSON() ([]byte, error) {
	if ref.fr != nil {
		return bytestr(ref.fr), nil
	}
	if ref.mr != nil {
		return bytestr(ref.mr), nil
	}
	if ref.cr != nil {
		return bytestr(ref.cr), nil
	}
	return nil, fmt.Errorf("should not all be nil")
}

func bytestr(r refs.Ref) []byte {
	return []byte("\"" + r.Ref() + "\"")
}

func (ref *BinaryRef) UnmarshalJSON(data []byte) error {
	// spew.Dump(string(data))
	return errors.Errorf("TODO:json")
}

func (ref BinaryRef) GetRef(t RefType) (refs.Ref, error) {
	hasT, err := ref.valid()
	if err != nil {
		return nil, errors.Wrap(err, "GetRef: invalid reference")
	}
	if hasT != t {
		return nil, errors.Errorf("GetRef: asked for type differs (has %d)", hasT)
	}
	// we could straight up return what is stored
	// but then we still have to assert afterwards if it really is what we want
	var ret refs.Ref
	switch t {
	case RefTypeFeed:
		ret = ref.fr
	case RefTypeMessage:
		ret = ref.mr
	case RefTypeContent:
		ret = ref.cr
	default:
		return nil, fmt.Errorf("GetRef: invalid ref type: %d", t)
	}
	return ret, nil
}

func NewBinaryRef(r refs.Ref) (BinaryRef, error) {
	return fromRef(r)
}

func fromRef(r refs.Ref) (BinaryRef, error) {
	var br BinaryRef
	switch tr := r.(type) {
	case refs.FeedRef:
		br.fr = &tr
	case refs.MessageRef:
		br.mr = &tr
	case ContentRef:
		br.cr = &tr
	default:
		return BinaryRef{}, fmt.Errorf("fromRef: invalid ref type: %T", r)
	}
	return br, nil
}

func refFromPubKey(pk ed25519.PublicKey) (*BinaryRef, error) {
	if len(pk) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key")
	}
	fr, err := refs.NewFeedRefFromBytes(pk, refs.RefAlgoFeedGabby)
	return &BinaryRef{
		fr: &fr,
	}, err
}

type BinRefExt struct{}

var _ codec.InterfaceExt = (*BinRefExt)(nil)

func (x BinRefExt) ConvertExt(v interface{}) interface{} {
	br, ok := v.(*BinaryRef)
	if !ok {
		panic(fmt.Sprintf("unsupported format expecting to decode into *BinaryRef; got %T", v))
	}
	refBytes, err := br.MarshalBinary()
	if err != nil {
		panic(err) //hrm...
	}
	return refBytes
}

func (x BinRefExt) UpdateExt(dst interface{}, src interface{}) {
	br, ok := dst.(*BinaryRef)
	if !ok {
		panic(fmt.Sprintf("unsupported format - expecting to decode into *BinaryRef; got %T", dst))
	}

	input, ok := src.([]byte)
	if !ok {
		panic(fmt.Sprintf("unsupported input format - expecting to decode from []byte; got %T", src))
	}

	err := br.UnmarshalBinary(input)
	if err != nil {
		panic(err)
	}

}
