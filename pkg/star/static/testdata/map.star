gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "map feature for testing",
    default = tpb.MapFields(
        enums = {
            0: tpb.TestEnum.TEST_ENUM_B,
            1: tpb.TestEnum.TEST_ENUM_A,
        },
        strings = {
            "foo": "bar",
            "zoo": "car",
        },
    ),
)
