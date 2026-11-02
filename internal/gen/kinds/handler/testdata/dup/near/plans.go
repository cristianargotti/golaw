package plansapi

// GetPlan serves GET /plans/{id}.
func (h *Handler) GetPlan(ctx context.Context, request apigen.GetPlanRequestObject) (apigen.GetPlanResponseObject, error) {
	wrap := gensupport.Prefix("plansapi: get plan")
	plan, err := h.uc.ReadPlan.Execute(ctx, growth.AccountID(request.Id))
	if err == nil {
		return apigen.GetPlan200JSONResponse{Account: string(plan.Account), Name: plan.Name, Archived: plan.Archived}, nil
	}
	if statusOf(err, statusRules()...) == http.StatusNotFound {
		return apigen.GetPlan404ApplicationProblemPlusJSONResponse{Title: http.StatusText(http.StatusNotFound), Status: http.StatusNotFound}, nil
	}
	return nil, wrap(err)
}
