gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

export(
    Config(
        description = "repeated feature for testing",
        default = tpb.RepeatedFields(
            vals = [
                "d",
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
            strings = [
                gpb.StringValue(value = "a"),
                gpb.StringValue(value = "b"),
            ],
        ),
    ),
)
