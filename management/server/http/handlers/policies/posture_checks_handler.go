package policies

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/netbirdio/netbird/management/server/account"
	nbcontext "github.com/netbirdio/netbird/management/server/context"
	"github.com/netbirdio/netbird/management/server/geolocation"
	"github.com/netbirdio/netbird/management/server/http/api"
	"github.com/netbirdio/netbird/management/server/http/util"
	"github.com/netbirdio/netbird/management/server/posture"
	"github.com/netbirdio/netbird/management/server/status"
)

// postureChecksHandler is a handler that returns posture checks of the account.
type postureChecksHandler struct {
	accountManager     account.Manager
	geolocationManager geolocation.Geolocation
}

func addPostureCheckEndpoint(accountManager account.Manager, locationManager geolocation.Geolocation, router *mux.Router) {
	postureCheckHandler := newPostureChecksHandler(accountManager, locationManager)
	router.HandleFunc("/posture-checks", postureCheckHandler.getAllPostureChecks).Methods("GET", "OPTIONS")
	router.HandleFunc("/posture-checks", postureCheckHandler.createPostureCheck).Methods("POST", "OPTIONS")
	router.HandleFunc("/posture-checks/{postureCheckId}", postureCheckHandler.updatePostureCheck).Methods("PUT", "OPTIONS")
	router.HandleFunc("/posture-checks/{postureCheckId}", postureCheckHandler.getPostureCheck).Methods("GET", "OPTIONS")
	router.HandleFunc("/posture-checks/{postureCheckId}", postureCheckHandler.deletePostureCheck).Methods("DELETE", "OPTIONS")
	addLocationsEndpoint(accountManager, locationManager, router)
}

// newPostureChecksHandler creates a new PostureChecks handler
func newPostureChecksHandler(accountManager account.Manager, geolocationManager geolocation.Geolocation) *postureChecksHandler {
	return &postureChecksHandler{
		accountManager:     accountManager,
		geolocationManager: geolocationManager,
	}
}

// getAllPostureChecks list for the account
func (p *postureChecksHandler) getAllPostureChecks(w http.ResponseWriter, r *http.Request) {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	accountID, userID := userAuth.AccountId, userAuth.UserId
	listPostureChecks, err := p.accountManager.ListPostureChecks(r.Context(), accountID, userID)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	postureChecks := make([]*api.PostureCheck, 0, len(listPostureChecks))
	for _, postureCheck := range listPostureChecks {
		postureChecks = append(postureChecks, postureCheck.ToAPIResponse())
	}

	util.WriteJSONObject(r.Context(), w, postureChecks)
}

// updatePostureCheck handles update to a posture check identified by a given ID
func (p *postureChecksHandler) updatePostureCheck(w http.ResponseWriter, r *http.Request) {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	accountID, userID := userAuth.AccountId, userAuth.UserId

	vars := mux.Vars(r)
	postureChecksID := vars["postureCheckId"]
	if len(postureChecksID) == 0 {
		util.WriteError(r.Context(), status.Errorf(status.InvalidArgument, "invalid posture checks ID"), w)
		return
	}

	_, err = p.accountManager.GetPostureChecks(r.Context(), accountID, postureChecksID, userID)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	p.savePostureChecks(w, r, accountID, userID, postureChecksID)
}

// createPostureCheck handles posture check creation request
func (p *postureChecksHandler) createPostureCheck(w http.ResponseWriter, r *http.Request) {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	accountID, userID := userAuth.AccountId, userAuth.UserId

	p.savePostureChecks(w, r, accountID, userID, "")
}

// getPostureCheck handles a posture check Get request identified by ID
func (p *postureChecksHandler) getPostureCheck(w http.ResponseWriter, r *http.Request) {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	accountID, userID := userAuth.AccountId, userAuth.UserId
	vars := mux.Vars(r)
	postureChecksID := vars["postureCheckId"]
	if len(postureChecksID) == 0 {
		util.WriteError(r.Context(), status.Errorf(status.InvalidArgument, "invalid posture checks ID"), w)
		return
	}

	postureChecks, err := p.accountManager.GetPostureChecks(r.Context(), accountID, postureChecksID, userID)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	util.WriteJSONObject(r.Context(), w, postureChecks.ToAPIResponse())
}

// deletePostureCheck handles posture check deletion request
func (p *postureChecksHandler) deletePostureCheck(w http.ResponseWriter, r *http.Request) {
	userAuth, err := nbcontext.GetUserAuthFromContext(r.Context())
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	accountID, userID := userAuth.AccountId, userAuth.UserId
	vars := mux.Vars(r)
	postureChecksID := vars["postureCheckId"]
	if len(postureChecksID) == 0 {
		util.WriteError(r.Context(), status.Errorf(status.InvalidArgument, "invalid posture checks ID"), w)
		return
	}

	if err = p.accountManager.DeletePostureChecks(r.Context(), accountID, postureChecksID, userID); err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	util.WriteJSONObject(r.Context(), w, util.EmptyObject{})
}

// savePostureChecks handles posture checks create and update
func (p *postureChecksHandler) savePostureChecks(w http.ResponseWriter, r *http.Request, accountID, userID, postureChecksID string) {
	var (
		err error
		req api.PostureCheckUpdate
	)

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteErrorResponse("couldn't parse JSON request", http.StatusBadRequest, w)
		return
	}

	if geoLocationCheck := req.Checks.GeoLocationCheck; geoLocationCheck != nil {
		if p.geolocationManager == nil {
			util.WriteError(r.Context(), status.Errorf(status.PreconditionFailed, "Geo location database is not initialized. "+
				"Check the self-hosted Geo database documentation at https://docs.netbird.io/selfhosted/geo-support"), w)
			return
		}
	}

	postureChecks, err := posture.NewChecksFromAPIPostureCheckUpdate(req, postureChecksID)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	postureChecks, err = p.accountManager.SavePostureChecks(r.Context(), accountID, userID, postureChecks)
	if err != nil {
		util.WriteError(r.Context(), err, w)
		return
	}

	util.WriteJSONObject(r.Context(), w, postureChecks.ToAPIResponse())
}
