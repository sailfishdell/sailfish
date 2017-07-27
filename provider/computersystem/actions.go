package actions


func SetupActions(s provider.ProviderRegisterer) {
	s.RegisterHandler(
		"POST:uri:"+s.GetBaseURI()+"/v1/Actions/ComputerSystem.Reset", // ?system=437XR1138R2,
		provider.RequestAdapter{Handle: makeHandlePost(s)},
	)
    return
}

func makeHandlePost(d domain.DDDFunctions) func(context.Context, *http.Request, []string, eh.UUID, *domain.RedfishTree, *domain.RedfishResource) error {
	return func(ctx context.Context, r *http.Request, privileges []string, cmdID eh.UUID, tree *domain.RedfishTree, requested *domain.RedfishResource) error {
		decoder := json.NewDecoder(r.Body)
		var lr LoginRequest
		err := decoder.Decode(&lr)

		if err != nil {
			return nil
		}

		// set up the session redfish resource
		event := eh.NewEvent(domain.HTTPCmdProcessedEvent,
			&domain.HTTPCmdProcessedData{
				CommandID: cmdID,
				Results:   retprops,
				Headers: map[string]string{
					"X-Auth-Token": tokenString,
					"Location":     sessionURI,
				},
			})

		d.GetEventHandler().HandleEvent(ctx, event)
		return nil
	}
}


