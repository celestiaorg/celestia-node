package tests

/*
func TestRPC(t *testing.T) {
	sw := swamp.NewSwamp(t)
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	la := fmt.Sprintf("http://%s", bridge.RPCServer.ListenAddr())

	client, closer, err := client.NewClient(ctx, la)
	require.NoError(t, err)
	t.Cleanup(closer.CloseAll)

	info := client.P2P.Peers()
	t.Log(info)
}

*/
