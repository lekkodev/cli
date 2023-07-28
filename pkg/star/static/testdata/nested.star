gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

export(
    description = "nested feature for testing",
    default = tpb.MultiLevel(
        primitive = "foo",
        sub_message = tpb.TestMessage(
            val = "bar",
            test_enum = tpb.TestEnum.TEST_ENUM_A,
            est = 0.12,
            off = gpb.BoolValue(value = True),
        ),
        nested = tpb.MultiLevel.Nested(nested_primitive = True),
        sub_external = gpb.StringValue(value = "external"),
    ),
)
