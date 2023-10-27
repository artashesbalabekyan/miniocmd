package miniocmd

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *bucketMetacache) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "bucket":
			z.bucket, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "bucket")
				return
			}
		case "caches":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "caches")
				return
			}
			if z.caches == nil {
				z.caches = make(map[string]metacache, zb0002)
			} else if len(z.caches) > 0 {
				for key := range z.caches {
					delete(z.caches, key)
				}
			}
			for zb0002 > 0 {
				zb0002--
				var za0001 string
				var za0002 metacache
				za0001, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "caches")
					return
				}
				err = za0002.DecodeMsg(dc)
				if err != nil {
					err = msgp.WrapError(err, "caches", za0001)
					return
				}
				z.caches[za0001] = za0002
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *bucketMetacache) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "bucket"
	err = en.Append(0x82, 0xa6, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.bucket)
	if err != nil {
		err = msgp.WrapError(err, "bucket")
		return
	}
	// write "caches"
	err = en.Append(0xa6, 0x63, 0x61, 0x63, 0x68, 0x65, 0x73)
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.caches)))
	if err != nil {
		err = msgp.WrapError(err, "caches")
		return
	}
	for za0001, za0002 := range z.caches {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "caches")
			return
		}
		err = za0002.EncodeMsg(en)
		if err != nil {
			err = msgp.WrapError(err, "caches", za0001)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *bucketMetacache) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "bucket"
	o = append(o, 0x82, 0xa6, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74)
	o = msgp.AppendString(o, z.bucket)
	// string "caches"
	o = append(o, 0xa6, 0x63, 0x61, 0x63, 0x68, 0x65, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.caches)))
	for za0001, za0002 := range z.caches {
		o = msgp.AppendString(o, za0001)
		o, err = za0002.MarshalMsg(o)
		if err != nil {
			err = msgp.WrapError(err, "caches", za0001)
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *bucketMetacache) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "bucket":
			z.bucket, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "bucket")
				return
			}
		case "caches":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "caches")
				return
			}
			if z.caches == nil {
				z.caches = make(map[string]metacache, zb0002)
			} else if len(z.caches) > 0 {
				for key := range z.caches {
					delete(z.caches, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 metacache
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "caches")
					return
				}
				bts, err = za0002.UnmarshalMsg(bts)
				if err != nil {
					err = msgp.WrapError(err, "caches", za0001)
					return
				}
				z.caches[za0001] = za0002
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *bucketMetacache) Msgsize() (s int) {
	s = 1 + 7 + msgp.StringPrefixSize + len(z.bucket) + 7 + msgp.MapHeaderSize
	if z.caches != nil {
		for za0001, za0002 := range z.caches {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + za0002.Msgsize()
		}
	}
	return
}
