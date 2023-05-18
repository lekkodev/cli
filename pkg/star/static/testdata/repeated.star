gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "repeated feature for testing",
    default = tpb.RepeatedFields(
        enums = [
            tpb.TestEnum.TEST_ENUM_A,
            tpb.TestEnum.TEST_ENUM_B,
        ],
        messages = [
            tpb.TestMessage(),
            tpb.TestMessage(),
        ],
        vals = [
            "a",
            "b",
            "c",
        ],
    ),
)
