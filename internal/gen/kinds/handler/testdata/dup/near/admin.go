package plansapi

// ArchivePlan serves POST /plans/{id}/archive.
func (h *Handler) ArchivePlan(ctx context.Context, request apigen.ArchivePlanRequestObject) (apigen.ArchivePlanResponseObject, error) {
	wrap := gensupport.Prefix("plansapi: archive plan")
	_, err := h.uc.ArchivePlan.Execute(ctx, growth.AccountID(request.Id))
	if err == nil {
		return apigen.ArchivePlan204Response{}, nil
	}
	if statusOf(err, statusRules()...) == http.StatusNotFound {
		return apigen.ArchivePlan404ApplicationProblemPlusJSONResponse{Title: http.StatusText(http.StatusNotFound), Status: http.StatusNotFound}, nil
	}
	return nil, wrap(err)
}
