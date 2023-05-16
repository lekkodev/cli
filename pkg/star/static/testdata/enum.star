gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "nested feature for testing",
    default = tpb.TestMessage(
        test_enum = tpb.TestEnum.TEST_ENUM_A,
    ),
)
