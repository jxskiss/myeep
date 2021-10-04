package myxds

import "github.com/golang/protobuf/proto"


func copyMessage(dst, src proto.Message) {
	buf, _ := proto.Marshal(src)
	_ = proto.Unmarshal(buf, dst)
}
