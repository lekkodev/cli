gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

export(
    Config(
        description = "map feature for testing",
        default = tpb.MapFields(
            strings = {
                "foo": "bar",
                "zoo": "car",
            },
            enums = {
                0: tpb.TestEnum.TEST_ENUM_B,
                1: tpb.TestEnum.TEST_ENUM_A,
            },
        ),
    ),
)
