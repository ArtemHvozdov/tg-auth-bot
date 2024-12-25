package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	//"strconv"
	"time"

	"github.com/ArtemHvozdov/tg-auth-bot/storage"

	"github.com/ethereum/go-ethereum/common"
	circuits "github.com/iden3/go-circuits/v2"
	auth "github.com/iden3/go-iden3-auth/v2"
	"github.com/iden3/go-iden3-auth/v2/pubsignals"
	"github.com/iden3/go-iden3-auth/v2/state"
	"github.com/iden3/iden3comm/v2/protocol"
)

const VerificationKeyPath = "verification_key.json"


type KeyLoader struct {
	Dir string
}

type AuthRequestData struct {
	Request protocol.AuthorizationRequestMessage
	UserID  int64
}

// Load keys from embedded FS
func (m KeyLoader) Load(id circuits.CircuitID) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("%s/%v/%s", m.Dir, id, VerificationKeyPath))
}

// Map for storing authentication requests
var requestMap = make(map[string]AuthRequestData)

// GenerateAuthRequest generates a new authentication request and returns it as a JSON object
func GenerateAuthRequest(userID int64, params storage.VerificationParams) ([]byte, error) {
	rURL := "https://b995-109-72-122-36.ngrok-free.app" // Updatesd with your actual URL
	sessionID := "1"                                     // Use unique session IDs in production
	//sessionID := strconv.Itoa(int(time.Now().UnixNano()))

	log.Println("Session ID in Generate Auth Request:", sessionID)
	CallbackURL := "/api/callback"
	Audience := "did:polygonid:polygon:amoy:2qQ68JkRcf3xrHPQPWZei3YeVzHPP58wYNxx2mEouR"

	// Forming a URI for callback
	uri := fmt.Sprintf("%s%s?sessionId=%s", rURL, CallbackURL, sessionID)

	log.Println("URI:", uri)

	// Create an authorization request
	var request protocol.AuthorizationRequestMessage = auth.CreateAuthorizationRequest("test flow", Audience, uri)

	// Adding a request for proof
	var mtpProofRequest protocol.ZeroKnowledgeProofRequest
	// mtpProofRequest.ID = 1
	// mtpProofRequest.CircuitID = string(circuits.AtomicQuerySigV2CircuitID)
	// mtpProofRequest.Query = map[string]interface{}{
	// 	"allowedIssuers": []string{"*"},
	// 	"credentialSubject": map[string]interface{}{
	// 		"birthday": map[string]interface{}{
	// 			"$lt": 20000101,
	// 		},
	// 	},
	// 	"context": "https://raw.githubusercontent.com/iden3/claim-schema-vocab/main/schemas/json-ld/kyc-v4.jsonld",
	// 	"type":    "KYCAgeCredential",
	// }


	mtpProofRequest.ID = params.ID
	mtpProofRequest.CircuitID = params.CircuitID
	mtpProofRequest.Query = params.Query

	request.Body.Scope = append(request.Body.Scope, mtpProofRequest)


	// Store auth request in map associated with session ID
	// requestMap[strconv.Itoa(sessionID)] = request

	requestMap[sessionID] = AuthRequestData{
        Request: request,
        UserID:  userID,
    }

	log.Println("reques map:", requestMap)
	
	// print request
	fmt.Println(request)
	
	msgBytes, _ := json.Marshal(request)

	// Returning a JSON object
	return msgBytes, nil
}


// Callback handles the callback from iden3
func Callback(w http.ResponseWriter, r *http.Request) bool {
	log.Println("Callback received")
	sessionID := r.URL.Query().Get("sessionId")

	log.Println("Session ID in Cal:", sessionID)

	tokenBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading token from request body:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return false
	}

	ethURL := "https://polygon-amoy.infura.io/v3/a1e81bcaca104bf9ad54f4e88b4c3554"
	contractAddress := "0x1a4cC30f2aA0377b0c3bc9848766D90cb4404124"
	resolverPrefix := "polygon:amoy"
	
	keyDIR := "./keys"

	log.Println("Callback func: map request:", requestMap)

	// Receiving authRequest by sessionID
	authRequest, ok := requestMap[sessionID]
	if !ok {
		log.Println("Session ID not found")
		http.Error(w, "Session not found", http.StatusNotFound)
		return false
	}

	userID := authRequest.UserID

	verificationKeyLoader := &KeyLoader{Dir: keyDIR}
	resolver := state.ETHResolver{
		RPCUrl:          ethURL,
		ContractAddress: common.HexToAddress(contractAddress),
	}

	resolverPrivado := state.ETHResolver{
		RPCUrl:          "https://rpc-mainnet.privado.id",
		ContractAddress: common.HexToAddress("0x975556428F077dB5877Ea2474D783D6C69233742"),
	}

	resolvers := map[string]pubsignals.StateResolver{
		resolverPrefix: resolver,
		"privado:main": resolverPrivado, 
	}

	verifier, err := auth.NewVerifier(verificationKeyLoader, resolvers, auth.WithIPFSGateway("https://ipfs.io"))
	if err != nil {
		log.Println("Error creating verifier:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return false
	}

	// Performing verification
	authResponse, err := verifier.FullVerify(
		r.Context(),
		string(tokenBytes),
		authRequest.Request,
		pubsignals.WithAcceptedStateTransitionDelay(time.Minute*5),
	)
	if err != nil {
		log.Println("Verification failed:", err)

		// Getting the user using the GetUser method
		_, exists := storage.GetUser(userID)
		if exists {
			// Update user status via UpdateField
			storage.UpdateField(userID, func(user *storage.UserVerification) {
				user.IsPending = false
				user.Verified = false
			})
		}

		http.Error(w, "Verification failed", http.StatusForbidden)
		return false
	}

	// Update the user status if verification is successful
	userData, exists := storage.GetUser(userID)
	if exists {
		storage.UpdateField(userID, func(user *storage.UserVerification) {
			user.IsPending = false
			user.Verified = true
		})
		log.Printf("User @%s (ID: %d) successfully verified via callback.", userData.Username, userID)
	}

	// Response to request with verification result
	resposeBytes, err := json.Marshal(authResponse)
	if err != nil {
		log.Println("Error serializing auth response:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return false
	}

	if resposeBytes == nil {
		log.Println("Response is empty")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else {
		log.Println("Response is here")
	}

	// w.WriteHeader(http.StatusOK)
	// w.Header().Set("Content-Type", "application/json")
	// w.Write(responseBytes)
	log.Println("Verification passed")

	// Logging user information
	log.Println("Information of new member:", userData)
	return true
}


func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Server is running. Welcome to the home page!")
}