package myxds

import (
	"fmt"

	"github.com/golang/protobuf/proto"
)

func copyMessage(dst, src proto.Message) {
	buf, _ := proto.Marshal(src)
	_ = proto.Unmarshal(buf, dst)
}

func getListenerName(port uint) string {
	return fmt.Sprintf("listener_%d", port)
}

func getVirtualHostName(port uint, domainGroupName string) string {
	return fmt.Sprintf("listener_%d_%s", port, domainGroupName)
}
