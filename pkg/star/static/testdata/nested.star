gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

result = feature(
    description = "nested feature for testing",
    default = tpb.MultiLevel(
        primitive = "foo",
        sub_message = tpb.TestMessage(val = "bar"),
        nested = tpb.MultiLevel.Nested(nested_primitive = True),
        sub_external = gpb.StringValue(value = "external"),
    ),
)
