gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "repeated feature for testing",
    default = tpb.RepeatedFields(
        vals = [
            "a",
            "b",
            "c",
        ],
        messages = [
            tpb.TestMessage(),
            tpb.TestMessage(),
        ],
        enums = [
            tpb.TestEnum.TEST_ENUM_A,
            tpb.TestEnum.TEST_ENUM_B,
        ],
    ),
)
