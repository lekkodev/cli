
google_protobuf = proto.package("google.protobuf")
internal_config_v1beta1 = proto.package("internal.config.v1beta1")

export(
    Config(
        description = "my feature description",
        default = internal_config_v1beta1.ProductMetadata(
            state = internal_config_v1beta1.ProductState.PRODUCT_STATE_UNSPECIFIED,
            description = "",
            time = google_protobuf.Timestamp(),
            friend = internal_config_v1beta1.Friend(),
            build = internal_config_v1beta1.Build(),
            sell = internal_config_v1beta1.Sell(),
        ),
    ),
)
