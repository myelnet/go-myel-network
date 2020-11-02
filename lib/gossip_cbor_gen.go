// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package rtmkt

import (
	"fmt"
	"io"

	peer "github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

func (t *GossipMessage) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{162}); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.PayloadCID (cid.Cid) (struct)
	if len("PayloadCID") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"PayloadCID\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("PayloadCID"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("PayloadCID")); err != nil {
		return err
	}

	if err := cbg.WriteCidBuf(scratch, w, t.PayloadCID); err != nil {
		return xerrors.Errorf("failed to write cid field t.PayloadCID: %w", err)
	}

	// t.SenderID (peer.ID) (string)
	if len("SenderID") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"SenderID\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("SenderID"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("SenderID")); err != nil {
		return err
	}

	if len(t.SenderID) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.SenderID was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.SenderID))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.SenderID)); err != nil {
		return err
	}
	return nil
}

func (t *GossipMessage) UnmarshalCBOR(r io.Reader) error {
	*t = GossipMessage{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajMap {
		return fmt.Errorf("cbor input should be of type map")
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("GossipMessage: map struct too large (%d)", extra)
	}

	var name string
	n := extra

	for i := uint64(0); i < n; i++ {

		{
			sval, err := cbg.ReadStringBuf(br, scratch)
			if err != nil {
				return err
			}

			name = string(sval)
		}

		switch name {
		// t.PayloadCID (cid.Cid) (struct)
		case "PayloadCID":

			{

				c, err := cbg.ReadCid(br)
				if err != nil {
					return xerrors.Errorf("failed to read cid field t.PayloadCID: %w", err)
				}

				t.PayloadCID = c

			}
			// t.SenderID (peer.ID) (string)
		case "SenderID":

			{
				sval, err := cbg.ReadStringBuf(br, scratch)
				if err != nil {
					return err
				}

				t.SenderID = peer.ID(sval)
			}

		default:
			return fmt.Errorf("unknown struct field %d: '%s'", i, name)
		}
	}

	return nil
}
