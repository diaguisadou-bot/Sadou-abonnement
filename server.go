package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Order represents one submission from the order form.
type Order struct {
	Nom       string    `json:"nom"`
	Tel       string    `json:"tel"`
	Service   string    `json:"service"`
	Formule   string    `json:"formule"`
	Paiement  string    `json:"paiement"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

const ordersFile = "orders.json"

var mu sync.Mutex

// AdminKey protects the /api/commandes listing endpoint.
// Set it via the ADMIN_KEY environment variable when deploying.
var adminKey = os.Getenv("ADMIN_KEY")

func loadOrders() ([]Order, error) {
	var orders []Order
	data, err := os.ReadFile(ordersFile)
	if os.IsNotExist(err) {
		return orders, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return orders, nil
	}
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func saveOrders(orders []Order) error {
	data, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ordersFile, data, 0644)
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if o.Nom == "" || o.Tel == "" || o.Service == "" {
		http.Error(w, "champs requis manquants (nom, tel, service)", http.StatusBadRequest)
		return
	}
	o.CreatedAt = time.Now()

	mu.Lock()
	defer mu.Unlock()

	orders, err := loadOrders()
	if err != nil {
		log.Println("loadOrders error:", err)
		http.Error(w, "erreur serveur", http.StatusInternalServerError)
		return
	}
	orders = append(orders, o)
	if err := saveOrders(orders); err != nil {
		log.Println("saveOrders error:", err)
		http.Error(w, "erreur serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Nouvelle commande: %s (%s) — %s / %s\n", o.Nom, o.Tel, o.Service, o.Formule)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleListOrders(w http.ResponseWriter, r *http.Request) {
	if adminKey == "" || r.URL.Query().Get("key") != adminKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	orders, err := loadOrders()
	if err != nil {
		http.Error(w, "erreur serveur", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h(w, r)
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/commande", withCORS(handleCreateOrder))
	mux.HandleFunc("/api/commandes", withCORS(handleListOrders))
	mux.Handle("/", http.FileServer(http.Dir(".")))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Serveur démarré sur http://localhost%s\n", addr)
	if adminKey == "" {
		log.Println("ATTENTION: ADMIN_KEY non défini — /api/commandes est désactivé tant que vous ne le définissez pas.")
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}
