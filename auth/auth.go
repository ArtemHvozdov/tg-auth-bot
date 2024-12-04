package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

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

// Load keys from embedded FS
func (m KeyLoader) Load(id circuits.CircuitID) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("%s/%v/%s", m.Dir, id, VerificationKeyPath))
}

// Map for storing authentication requests
var requestMap = make(map[string]interface{})

// GetAuthRequest generates a new authentication request
// func GetAuthRequest(w http.ResponseWriter, r *http.Request) {
// 	rURL := " https://4977-91-244-53-233.ngrok-free.app" // Update with your actual URL
// 	sessionID := 1                                 // Use unique session IDs in production
// 	CallbackURL := "/api/callback"
// 	Audience := "did:polygonid:polygon:amoy:2qQ68JkRcf3xrHPQPWZei3YeVzHPP58wYNxx2mEouR"

// 	uri := fmt.Sprintf("%s%s?sessionId=%s", rURL, CallbackURL, strconv.Itoa(sessionID))

// 	var request protocol.AuthorizationRequestMessage = auth.CreateAuthorizationRequest("test flow", Audience, uri)

// 	// Add request for proof
// 	var mtpProofRequest protocol.ZeroKnowledgeProofRequest
// 	mtpProofRequest.ID = 1
// 	mtpProofRequest.CircuitID = string(circuits.AtomicQuerySigV2CircuitID)
// 	mtpProofRequest.Query = map[string]interface{}{
// 		"allowedIssuers": []string{"*"},
// 		"credentialSubject": map[string]interface{}{
// 			"birthday": map[string]interface{}{
// 				"$lt": 20000101,
// 			},
// 		},
// 		"context": "https://raw.githubusercontent.com/iden3/claim-schema-vocab/main/schemas/json-ld/kyc-v4.jsonld",
// 		"type":    "KYCAgeCredential",
// 	}
// 	request.Body.Scope = append(request.Body.Scope, mtpProofRequest)

// 	// Store auth request
// 	requestMap[strconv.Itoa(sessionID)] = request

// 	msgBytes, _ := json.Marshal(request)

// 	err := qrcode.WriteFile(string(msgBytes), qrcode.Medium, 256, "qr.png")
// 	if err != nil {
// 		log.Printf("Error generating QR code: %v", err)
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	w.Write(msgBytes)
// }

// GenerateAuthRequest generates a new authentication request and returns it as a JSON object
func GenerateAuthRequest() ([]byte, error) {
	rURL := "https://e9fd-109-72-122-36.ngrok-free.app" // Updatesd with your actual URL
	sessionID := 1                                     // Use unique session IDs in production
	CallbackURL := "/api/callback"
	Audience := "did:polygonid:polygon:amoy:2qQ68JkRcf3xrHPQPWZei3YeVzHPP58wYNxx2mEouR"

	// Формируем URI для callback
	uri := fmt.Sprintf("%s%s?sessionId=%s", rURL, CallbackURL, strconv.Itoa(sessionID))

	log.Println("URI:", uri)

	// Создаем запрос на авторизацию
	var request protocol.AuthorizationRequestMessage = auth.CreateAuthorizationRequest("test flow", Audience, uri)

	// Добавляем запрос для доказательства
	var mtpProofRequest protocol.ZeroKnowledgeProofRequest
	mtpProofRequest.ID = 1
	mtpProofRequest.CircuitID = string(circuits.AtomicQuerySigV2CircuitID)
	mtpProofRequest.Query = map[string]interface{}{
		"allowedIssuers": []string{"*"},
		"credentialSubject": map[string]interface{}{
			"birthday": map[string]interface{}{
				"$lt": 20000101,
			},
		},
		"context": "https://raw.githubusercontent.com/iden3/claim-schema-vocab/main/schemas/json-ld/kyc-v4.jsonld",
		"type":    "KYCAgeCredential",
	}
	request.Body.Scope = append(request.Body.Scope, mtpProofRequest)

	// Store auth request in map associated with session ID
	requestMap[strconv.Itoa(sessionID)] = request
	
	// print request
	fmt.Println(request)
	
	msgBytes, _ := json.Marshal(request)

	// Возвращаем JSON объект
	return msgBytes, nil
}

// func CreateAuthorizationRequest(s, Audience, uri string) {
// 	panic("unimplemented")
// }

// Callback handles the callback from iden3
func Callback(w http.ResponseWriter, r *http.Request) {
	log.Println("Callback received")
	sessionID := r.URL.Query().Get("sessionId")

	tokenBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading token from request body:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ethURL := "https://polygon-amoy.infura.io/v3/<INFURA_KEY>"
	contractAddress := "0x1a4cC30f2aA0377b0c3bc9848766D90cb4404124"
	resolverPrefix := "polygon:amoy"
	keyDIR := "./keys"

	authRequest, ok := requestMap[sessionID]
	if !ok {
		log.Println("Session ID not found")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	verificationKeyLoader := &KeyLoader{Dir: keyDIR}
	resolver := state.ETHResolver{
		RPCUrl:          ethURL,
		ContractAddress: common.HexToAddress(contractAddress),
	}
	resolvers := map[string]pubsignals.StateResolver{
		resolverPrefix: resolver,
	}

	verifier, err := auth.NewVerifier(verificationKeyLoader, resolvers, auth.WithIPFSGateway("https://ipfs.io"))
	if err != nil {
		log.Println("Error creating verifier:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	authResponse, err := verifier.FullVerify(
		r.Context(),
		string(tokenBytes),
		authRequest.(protocol.AuthorizationRequestMessage),
		pubsignals.WithAcceptedStateTransitionDelay(time.Minute*5),
	)
	if err != nil {
		log.Println("Verification failed:", err)
		http.Error(w, "Verification failed", http.StatusForbidden)
		return
	}

	responseBytes, err := json.Marshal(authResponse)
	if err != nil {
		log.Println("Error serializing auth response:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)
	log.Println("Verification passed")
}

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Server is running. Welcome to the home page!")
}