package main

func registerHandlers() {
	r := router
	r.register(Ping, handlePing)
	r.register(AuthorizeRequest, handleAuthorizeRequest)
	r.register(SelectChanelRequest, handleSelectChanelRequest)
	r.register(SelectServerRequest, handleSelectServerRequest)
	r.register(CreateCharacterRequest, handleCreateCharacterRequest)
	r.register(EnterGameRequset, handleEnterGameRequest)
	r.register(ClientLoadVerification, handleClientVerificationNoop)
	r.register(ClientPreLocationVerification, handleClientVerificationNoop)
	r.register(ClientIdleTick, handleClientVerificationNoop)
	r.register(ClientUiActionRequest, handleClientVerificationNoop)
	r.register(U6072, handleClientVerificationNoop)
	r.register(U8215, handleClientVerificationNoop)
	r.register(U8226, handleClientVerificationNoop)
}

func handleClientVerificationNoop(c *Client, p *PacketIn) {}
