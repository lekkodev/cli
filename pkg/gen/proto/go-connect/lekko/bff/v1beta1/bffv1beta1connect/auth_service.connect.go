// Copyright 2022 Lekko Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: lekko/bff/v1beta1/auth_service.proto

package bffv1beta1connect

import (
	context "context"
	errors "errors"
	connect_go "github.com/bufbuild/connect-go"
	v1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect_go.IsAtLeastVersion0_1_0

const (
	// AuthServiceName is the fully-qualified name of the AuthService service.
	AuthServiceName = "lekko.bff.v1beta1.AuthService"
)

// AuthServiceClient is a client for the lekko.bff.v1beta1.AuthService service.
type AuthServiceClient interface {
	// We will return required auth info in a cookie
	// that is sent to the same origin for other requests
	// inside the bff service.
	Login(context.Context, *connect_go.Request[v1beta1.LoginRequest]) (*connect_go.Response[v1beta1.LoginResponse], error)
	// Logout will expire the user's cookie.
	Logout(context.Context, *connect_go.Request[v1beta1.LogoutRequest]) (*connect_go.Response[v1beta1.LogoutResponse], error)
	// Returns a response indicating if the account already existed.
	RegisterUser(context.Context, *connect_go.Request[v1beta1.RegisterUserRequest]) (*connect_go.Response[v1beta1.RegisterUserResponse], error)
	// An rpc that a 3rd party device makes to our backend to obtain device and user
	// codes to complete device oauth.
	GetDeviceCode(context.Context, *connect_go.Request[v1beta1.GetDeviceCodeRequest]) (*connect_go.Response[v1beta1.GetDeviceCodeResponse], error)
	// An rpc that a 3rd party device polls to obtain an access token once
	// the user has completed authentication through a browser-based user agent.
	GetAccessToken(context.Context, *connect_go.Request[v1beta1.GetAccessTokenRequest]) (*connect_go.Response[v1beta1.GetAccessTokenResponse], error)
}

// NewAuthServiceClient constructs a client for the lekko.bff.v1beta1.AuthService service. By
// default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses,
// and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewAuthServiceClient(httpClient connect_go.HTTPClient, baseURL string, opts ...connect_go.ClientOption) AuthServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &authServiceClient{
		login: connect_go.NewClient[v1beta1.LoginRequest, v1beta1.LoginResponse](
			httpClient,
			baseURL+"/lekko.bff.v1beta1.AuthService/Login",
			opts...,
		),
		logout: connect_go.NewClient[v1beta1.LogoutRequest, v1beta1.LogoutResponse](
			httpClient,
			baseURL+"/lekko.bff.v1beta1.AuthService/Logout",
			opts...,
		),
		registerUser: connect_go.NewClient[v1beta1.RegisterUserRequest, v1beta1.RegisterUserResponse](
			httpClient,
			baseURL+"/lekko.bff.v1beta1.AuthService/RegisterUser",
			opts...,
		),
		getDeviceCode: connect_go.NewClient[v1beta1.GetDeviceCodeRequest, v1beta1.GetDeviceCodeResponse](
			httpClient,
			baseURL+"/lekko.bff.v1beta1.AuthService/GetDeviceCode",
			opts...,
		),
		getAccessToken: connect_go.NewClient[v1beta1.GetAccessTokenRequest, v1beta1.GetAccessTokenResponse](
			httpClient,
			baseURL+"/lekko.bff.v1beta1.AuthService/GetAccessToken",
			opts...,
		),
	}
}

// authServiceClient implements AuthServiceClient.
type authServiceClient struct {
	login          *connect_go.Client[v1beta1.LoginRequest, v1beta1.LoginResponse]
	logout         *connect_go.Client[v1beta1.LogoutRequest, v1beta1.LogoutResponse]
	registerUser   *connect_go.Client[v1beta1.RegisterUserRequest, v1beta1.RegisterUserResponse]
	getDeviceCode  *connect_go.Client[v1beta1.GetDeviceCodeRequest, v1beta1.GetDeviceCodeResponse]
	getAccessToken *connect_go.Client[v1beta1.GetAccessTokenRequest, v1beta1.GetAccessTokenResponse]
}

// Login calls lekko.bff.v1beta1.AuthService.Login.
func (c *authServiceClient) Login(ctx context.Context, req *connect_go.Request[v1beta1.LoginRequest]) (*connect_go.Response[v1beta1.LoginResponse], error) {
	return c.login.CallUnary(ctx, req)
}

// Logout calls lekko.bff.v1beta1.AuthService.Logout.
func (c *authServiceClient) Logout(ctx context.Context, req *connect_go.Request[v1beta1.LogoutRequest]) (*connect_go.Response[v1beta1.LogoutResponse], error) {
	return c.logout.CallUnary(ctx, req)
}

// RegisterUser calls lekko.bff.v1beta1.AuthService.RegisterUser.
func (c *authServiceClient) RegisterUser(ctx context.Context, req *connect_go.Request[v1beta1.RegisterUserRequest]) (*connect_go.Response[v1beta1.RegisterUserResponse], error) {
	return c.registerUser.CallUnary(ctx, req)
}

// GetDeviceCode calls lekko.bff.v1beta1.AuthService.GetDeviceCode.
func (c *authServiceClient) GetDeviceCode(ctx context.Context, req *connect_go.Request[v1beta1.GetDeviceCodeRequest]) (*connect_go.Response[v1beta1.GetDeviceCodeResponse], error) {
	return c.getDeviceCode.CallUnary(ctx, req)
}

// GetAccessToken calls lekko.bff.v1beta1.AuthService.GetAccessToken.
func (c *authServiceClient) GetAccessToken(ctx context.Context, req *connect_go.Request[v1beta1.GetAccessTokenRequest]) (*connect_go.Response[v1beta1.GetAccessTokenResponse], error) {
	return c.getAccessToken.CallUnary(ctx, req)
}

// AuthServiceHandler is an implementation of the lekko.bff.v1beta1.AuthService service.
type AuthServiceHandler interface {
	// We will return required auth info in a cookie
	// that is sent to the same origin for other requests
	// inside the bff service.
	Login(context.Context, *connect_go.Request[v1beta1.LoginRequest]) (*connect_go.Response[v1beta1.LoginResponse], error)
	// Logout will expire the user's cookie.
	Logout(context.Context, *connect_go.Request[v1beta1.LogoutRequest]) (*connect_go.Response[v1beta1.LogoutResponse], error)
	// Returns a response indicating if the account already existed.
	RegisterUser(context.Context, *connect_go.Request[v1beta1.RegisterUserRequest]) (*connect_go.Response[v1beta1.RegisterUserResponse], error)
	// An rpc that a 3rd party device makes to our backend to obtain device and user
	// codes to complete device oauth.
	GetDeviceCode(context.Context, *connect_go.Request[v1beta1.GetDeviceCodeRequest]) (*connect_go.Response[v1beta1.GetDeviceCodeResponse], error)
	// An rpc that a 3rd party device polls to obtain an access token once
	// the user has completed authentication through a browser-based user agent.
	GetAccessToken(context.Context, *connect_go.Request[v1beta1.GetAccessTokenRequest]) (*connect_go.Response[v1beta1.GetAccessTokenResponse], error)
}

// NewAuthServiceHandler builds an HTTP handler from the service implementation. It returns the path
// on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewAuthServiceHandler(svc AuthServiceHandler, opts ...connect_go.HandlerOption) (string, http.Handler) {
	mux := http.NewServeMux()
	mux.Handle("/lekko.bff.v1beta1.AuthService/Login", connect_go.NewUnaryHandler(
		"/lekko.bff.v1beta1.AuthService/Login",
		svc.Login,
		opts...,
	))
	mux.Handle("/lekko.bff.v1beta1.AuthService/Logout", connect_go.NewUnaryHandler(
		"/lekko.bff.v1beta1.AuthService/Logout",
		svc.Logout,
		opts...,
	))
	mux.Handle("/lekko.bff.v1beta1.AuthService/RegisterUser", connect_go.NewUnaryHandler(
		"/lekko.bff.v1beta1.AuthService/RegisterUser",
		svc.RegisterUser,
		opts...,
	))
	mux.Handle("/lekko.bff.v1beta1.AuthService/GetDeviceCode", connect_go.NewUnaryHandler(
		"/lekko.bff.v1beta1.AuthService/GetDeviceCode",
		svc.GetDeviceCode,
		opts...,
	))
	mux.Handle("/lekko.bff.v1beta1.AuthService/GetAccessToken", connect_go.NewUnaryHandler(
		"/lekko.bff.v1beta1.AuthService/GetAccessToken",
		svc.GetAccessToken,
		opts...,
	))
	return "/lekko.bff.v1beta1.AuthService/", mux
}

// UnimplementedAuthServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedAuthServiceHandler struct{}

func (UnimplementedAuthServiceHandler) Login(context.Context, *connect_go.Request[v1beta1.LoginRequest]) (*connect_go.Response[v1beta1.LoginResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("lekko.bff.v1beta1.AuthService.Login is not implemented"))
}

func (UnimplementedAuthServiceHandler) Logout(context.Context, *connect_go.Request[v1beta1.LogoutRequest]) (*connect_go.Response[v1beta1.LogoutResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("lekko.bff.v1beta1.AuthService.Logout is not implemented"))
}

func (UnimplementedAuthServiceHandler) RegisterUser(context.Context, *connect_go.Request[v1beta1.RegisterUserRequest]) (*connect_go.Response[v1beta1.RegisterUserResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("lekko.bff.v1beta1.AuthService.RegisterUser is not implemented"))
}

func (UnimplementedAuthServiceHandler) GetDeviceCode(context.Context, *connect_go.Request[v1beta1.GetDeviceCodeRequest]) (*connect_go.Response[v1beta1.GetDeviceCodeResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("lekko.bff.v1beta1.AuthService.GetDeviceCode is not implemented"))
}

func (UnimplementedAuthServiceHandler) GetAccessToken(context.Context, *connect_go.Request[v1beta1.GetAccessTokenRequest]) (*connect_go.Response[v1beta1.GetAccessTokenResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("lekko.bff.v1beta1.AuthService.GetAccessToken is not implemented"))
}
