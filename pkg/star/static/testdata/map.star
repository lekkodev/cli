gpb = proto.package("google.protobuf")
tpb = proto.package("testproto.v1beta1")

# Note: protobuf maps are unordered, so a round trip static parsing
# is not stable (we may reorder the elements of the map).
result = feature(
    description = "map feature for testing",
    default = tpb.MapFields(
        enums = {1: tpb.TestEnum.TEST_ENUM_A},
        strings = {"foo": "bar"},
    ),
)
