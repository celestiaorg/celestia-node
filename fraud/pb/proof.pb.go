// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: fraud/pb/proof.proto

package fraud_pb

import (
	fmt "fmt"
	io "io"
	math "math"
	math_bits "math/bits"

	proto "github.com/gogo/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type FraudMessageRequest struct {
	RequestedProofType []string `protobuf:"bytes,1,rep,name=RequestedProofType,proto3" json:"RequestedProofType,omitempty"`
}

func (m *FraudMessageRequest) Reset()         { *m = FraudMessageRequest{} }
func (m *FraudMessageRequest) String() string { return proto.CompactTextString(m) }
func (*FraudMessageRequest) ProtoMessage()    {}
func (*FraudMessageRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_318cb87a8bb2d394, []int{0}
}
func (m *FraudMessageRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *FraudMessageRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_FraudMessageRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *FraudMessageRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FraudMessageRequest.Merge(m, src)
}
func (m *FraudMessageRequest) XXX_Size() int {
	return m.Size()
}
func (m *FraudMessageRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_FraudMessageRequest.DiscardUnknown(m)
}

var xxx_messageInfo_FraudMessageRequest proto.InternalMessageInfo

func (m *FraudMessageRequest) GetRequestedProofType() []string {
	if m != nil {
		return m.RequestedProofType
	}
	return nil
}

type ProofResponse struct {
	Type  string   `protobuf:"bytes,1,opt,name=Type,proto3" json:"Type,omitempty"`
	Value [][]byte `protobuf:"bytes,2,rep,name=Value,proto3" json:"Value,omitempty"`
}

func (m *ProofResponse) Reset()         { *m = ProofResponse{} }
func (m *ProofResponse) String() string { return proto.CompactTextString(m) }
func (*ProofResponse) ProtoMessage()    {}
func (*ProofResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_318cb87a8bb2d394, []int{1}
}
func (m *ProofResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ProofResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ProofResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ProofResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ProofResponse.Merge(m, src)
}
func (m *ProofResponse) XXX_Size() int {
	return m.Size()
}
func (m *ProofResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ProofResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ProofResponse proto.InternalMessageInfo

func (m *ProofResponse) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *ProofResponse) GetValue() [][]byte {
	if m != nil {
		return m.Value
	}
	return nil
}

type FraudMessageResponse struct {
	Proofs []*ProofResponse `protobuf:"bytes,1,rep,name=Proofs,proto3" json:"Proofs,omitempty"`
}

func (m *FraudMessageResponse) Reset()         { *m = FraudMessageResponse{} }
func (m *FraudMessageResponse) String() string { return proto.CompactTextString(m) }
func (*FraudMessageResponse) ProtoMessage()    {}
func (*FraudMessageResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_318cb87a8bb2d394, []int{2}
}
func (m *FraudMessageResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *FraudMessageResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_FraudMessageResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *FraudMessageResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FraudMessageResponse.Merge(m, src)
}
func (m *FraudMessageResponse) XXX_Size() int {
	return m.Size()
}
func (m *FraudMessageResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_FraudMessageResponse.DiscardUnknown(m)
}

var xxx_messageInfo_FraudMessageResponse proto.InternalMessageInfo

func (m *FraudMessageResponse) GetProofs() []*ProofResponse {
	if m != nil {
		return m.Proofs
	}
	return nil
}

func init() {
	proto.RegisterType((*FraudMessageRequest)(nil), "fraud.pb.FraudMessageRequest")
	proto.RegisterType((*ProofResponse)(nil), "fraud.pb.ProofResponse")
	proto.RegisterType((*FraudMessageResponse)(nil), "fraud.pb.FraudMessageResponse")
}

func init() { proto.RegisterFile("fraud/pb/proof.proto", fileDescriptor_318cb87a8bb2d394) }

var fileDescriptor_318cb87a8bb2d394 = []byte{
	// 203 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x49, 0x2b, 0x4a, 0x2c,
	0x4d, 0xd1, 0x2f, 0x48, 0xd2, 0x2f, 0x28, 0xca, 0xcf, 0x4f, 0xd3, 0x2b, 0x28, 0xca, 0x2f, 0xc9,
	0x17, 0xe2, 0x00, 0x8b, 0xea, 0x15, 0x24, 0x29, 0xb9, 0x72, 0x09, 0xbb, 0x81, 0xd8, 0xbe, 0xa9,
	0xc5, 0xc5, 0x89, 0xe9, 0xa9, 0x41, 0xa9, 0x85, 0xa5, 0xa9, 0xc5, 0x25, 0x42, 0x7a, 0x5c, 0x42,
	0x50, 0x66, 0x6a, 0x4a, 0x00, 0x48, 0x63, 0x48, 0x65, 0x41, 0xaa, 0x04, 0xa3, 0x02, 0xb3, 0x06,
	0x67, 0x10, 0x16, 0x19, 0x25, 0x4b, 0x2e, 0x5e, 0x30, 0x27, 0x28, 0xb5, 0xb8, 0x20, 0x3f, 0xaf,
	0x38, 0x55, 0x48, 0x88, 0x8b, 0x05, 0xaa, 0x85, 0x51, 0x83, 0x33, 0x08, 0xcc, 0x16, 0x12, 0xe1,
	0x62, 0x0d, 0x4b, 0xcc, 0x29, 0x4d, 0x95, 0x60, 0x52, 0x60, 0xd6, 0xe0, 0x09, 0x82, 0x70, 0x94,
	0xdc, 0xb9, 0x44, 0x50, 0x5d, 0x00, 0x35, 0x41, 0x9f, 0x8b, 0x0d, 0x6c, 0x64, 0x31, 0xd8, 0x5a,
	0x6e, 0x23, 0x71, 0x3d, 0x98, 0xa3, 0xf5, 0x50, 0xac, 0x0a, 0x82, 0x2a, 0x73, 0x92, 0x38, 0xf1,
	0x48, 0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6, 0x07, 0x8f, 0xe4, 0x18, 0x27, 0x3c, 0x96, 0x63, 0xb8,
	0xf0, 0x58, 0x8e, 0xe1, 0xc6, 0x63, 0x39, 0x86, 0x24, 0x36, 0xb0, 0xaf, 0x8d, 0x01, 0x01, 0x00,
	0x00, 0xff, 0xff, 0xa2, 0xc2, 0x65, 0x25, 0x0d, 0x01, 0x00, 0x00,
}

func (m *FraudMessageRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *FraudMessageRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *FraudMessageRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.RequestedProofType) > 0 {
		for iNdEx := len(m.RequestedProofType) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.RequestedProofType[iNdEx])
			copy(dAtA[i:], m.RequestedProofType[iNdEx])
			i = encodeVarintProof(dAtA, i, uint64(len(m.RequestedProofType[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *ProofResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ProofResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ProofResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Value) > 0 {
		for iNdEx := len(m.Value) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Value[iNdEx])
			copy(dAtA[i:], m.Value[iNdEx])
			i = encodeVarintProof(dAtA, i, uint64(len(m.Value[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Type) > 0 {
		i -= len(m.Type)
		copy(dAtA[i:], m.Type)
		i = encodeVarintProof(dAtA, i, uint64(len(m.Type)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *FraudMessageResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *FraudMessageResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *FraudMessageResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Proofs) > 0 {
		for iNdEx := len(m.Proofs) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Proofs[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintProof(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintProof(dAtA []byte, offset int, v uint64) int {
	offset -= sovProof(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *FraudMessageRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.RequestedProofType) > 0 {
		for _, s := range m.RequestedProofType {
			l = len(s)
			n += 1 + l + sovProof(uint64(l))
		}
	}
	return n
}

func (m *ProofResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Type)
	if l > 0 {
		n += 1 + l + sovProof(uint64(l))
	}
	if len(m.Value) > 0 {
		for _, b := range m.Value {
			l = len(b)
			n += 1 + l + sovProof(uint64(l))
		}
	}
	return n
}

func (m *FraudMessageResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Proofs) > 0 {
		for _, e := range m.Proofs {
			l = e.Size()
			n += 1 + l + sovProof(uint64(l))
		}
	}
	return n
}

func sovProof(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozProof(x uint64) (n int) {
	return sovProof(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *FraudMessageRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProof
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: FraudMessageRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: FraudMessageRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RequestedProofType", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProof
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProof
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProof
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RequestedProofType = append(m.RequestedProofType, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProof(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthProof
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ProofResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProof
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ProofResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ProofResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProof
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProof
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProof
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Type = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProof
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthProof
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthProof
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Value = append(m.Value, make([]byte, postIndex-iNdEx))
			copy(m.Value[len(m.Value)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProof(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthProof
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *FraudMessageResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProof
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: FraudMessageResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: FraudMessageResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Proofs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProof
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProof
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProof
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Proofs = append(m.Proofs, &ProofResponse{})
			if err := m.Proofs[len(m.Proofs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProof(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthProof
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipProof(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowProof
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowProof
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowProof
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthProof
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupProof
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthProof
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthProof        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowProof          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupProof = fmt.Errorf("proto: unexpected end of group")
)
