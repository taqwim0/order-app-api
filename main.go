package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

var db *sql.DB
var jwtSecret []byte
var coreAPI coreapi.Client

type Credentials struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	UserName string `json:"username"`
	jwt.StandardClaims
}

func init() {
	err := godotenv.Load("app.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var errDb error
	db, errDb = sql.Open("postgres", psqlInfo)
	if errDb != nil {
		log.Fatalf("Error connecting to the database: %v", errDb)
	}

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	coreAPI.New(os.Getenv("MIDTRANS_SERVER_KEY"), midtrans.Sandbox)
}

func main() {
	router := mux.NewRouter()

	// User API
	router.HandleFunc("/login", Login).Methods("POST")
	router.HandleFunc("/welcome", Welcome).Methods("GET")
	router.HandleFunc("/refresh", Refresh).Methods("POST")

	// Product API
	router.HandleFunc("/products", GetProducts).Methods("GET")

	// Cart API
	router.HandleFunc("/cart/{user_id}", GetCart).Methods("GET")
	router.HandleFunc("/cart/add/{user_id}", AddToCart).Methods("POST")
	router.HandleFunc("/cart/delete/{cart_id}", DeleteFromCart).Methods("DELETE")

	// Payment API
	router.HandleFunc("/payment/bill/{cart_id}", CreatePaymentBill).Methods("POST")
	router.HandleFunc("/payment/status/{cart_id}", GetPaymentStatus).Methods("GET")

	log.Println("order-app-api listen and serve :8080")
	log.Fatal(http.ListenAndServe(":8000", router))
}

func Login(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var storedPassword string
	err = db.QueryRow("SELECT user_password FROM order_app_api_user WHERE user_name=$1", creds.UserName).Scan(&storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	if creds.Password != storedPassword {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(10 * time.Minute)
	claims := &Claims{
		UserName: creds.UserName,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	log.Printf("current token: %+v", &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})
}

func Auth(w http.ResponseWriter, r *http.Request) (*Claims, error) {
	cookie, err := r.Cookie("token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return &Claims{}, errors.New("Unauthorized")
		}
		http.Error(w, "Bad request", http.StatusBadRequest)
		return &Claims{}, errors.New("Bad request")
	}

	tokenStr := cookie.Value
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return &Claims{}, errors.New("Unauthorized")
		}
		http.Error(w, "Bad request", http.StatusBadRequest)
		return &Claims{}, errors.New("Bad request")
	}
	if !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return &Claims{}, errors.New("Unauthorized")
	}

	return &Claims{
		UserName: claims.UserName,
	}, nil
}

func Welcome(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}

	w.Write([]byte(fmt.Sprintf("Welcome, %s!", claims.UserName)))

}

func Refresh(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}

	expirationTime := time.Now().Add(5 * time.Minute)
	claims.ExpiresAt = expirationTime.Unix()
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := newToken.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	log.Printf("refresh token: %+v", &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})
}

type Product struct {
	ProductID    int    `json:"product_id"`
	ProductName  string `json:"product_name"`
	ProductPrice int    `json:"product_price"`
}

func GetProducts(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}
	currentUser := claims.UserName

	rows, err := db.Query("SELECT product_id, product_name, product_price FROM order_app_api_products")
	if err != nil {
		http.Error(w, "Failed to execute query", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var product Product
		err := rows.Scan(&product.ProductID, &product.ProductName, &product.ProductPrice)
		if err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		products = append(products, product)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentUser)
	json.NewEncoder(w).Encode(products)
}

type Cart struct {
	CartID         int    `json:"cart_id"`
	UserID         int    `json:"user_id"`
	ProductID      int    `json:"product_id"`
	ProductName    string `json:"product_name"`
	ProductPrice   int    `json:"product_price"`
	CartTotalPrice int    `json:"cart_total_price"`
}

type CartRequestBody struct {
	ProductID int `json:"product_id"`
}

func GetCart(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}
	currentUser := claims.UserName

	params := mux.Vars(r)
	userID := params["user_id"]

	var cart Cart
	err = db.QueryRow("SELECT cart_id, user_id, product_id, product_name, product_price, cart_total_price FROM order_app_api_cart WHERE user_id=$1", userID).Scan(&cart.CartID, &cart.UserID, &cart.ProductID, &cart.ProductName, &cart.ProductPrice, &cart.CartTotalPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Cart not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentUser)
	json.NewEncoder(w).Encode(cart)
}

func AddToCart(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}
	currentUser := claims.UserName

	params := mux.Vars(r)
	userID := params["user_id"]

	var cartReqBody CartRequestBody
	var cart Cart

	err = json.NewDecoder(r.Body).Decode(&cartReqBody)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var productName string
	var productPrice int

	err = db.QueryRow("SELECT product_name, product_price FROM order_app_api_products WHERE product_id=$1", cartReqBody.ProductID).Scan(&productName, &productPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	userIDStr, _ := strconv.Atoi(userID)

	cart.UserID = userIDStr
	cart.ProductID = cartReqBody.ProductID
	cart.ProductName = productName
	cart.ProductPrice = productPrice
	cart.CartTotalPrice = productPrice

	err = db.QueryRow("INSERT INTO order_app_api_cart (user_id, product_id, product_name, product_price, cart_total_price) VALUES ($1, $2, $3, $4, $5) RETURNING cart_id",
		cart.UserID, cart.ProductID, cart.ProductName, cart.ProductPrice, cart.CartTotalPrice).Scan(&cart.CartID)
	if err != nil {
		fmt.Println("err db: ", err)
		http.Error(w, "Failed to insert cart", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentUser)
	json.NewEncoder(w).Encode(cart)
}

func DeleteFromCart(w http.ResponseWriter, r *http.Request) {
	claims, err := Auth(w, r)
	if err != nil {
		return
	}
	currentUser := claims.UserName

	params := mux.Vars(r)
	cartID := params["cart_id"]

	_, err = db.Exec("DELETE FROM order_app_api_cart WHERE cart_id=$1", cartID)
	if err != nil {
		http.Error(w, "Failed to delete cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currentUser)
}

type PaymentBill struct {
	CartTotalPrice string `json:"cart_total_price"`
	TransactionID  string `json:"transaction_id"`
	ProductName    string `json:"product_name"`
}

func CreatePaymentBill(w http.ResponseWriter, r *http.Request) {
	claims, errAuth := Auth(w, r)
	if errAuth != nil {
		return
	}
	currentUser := claims.UserName

	params := mux.Vars(r)
	cartID := params["cart_id"]

	var paymentBill PaymentBill

	errDecode := json.NewDecoder(r.Body).Decode(&paymentBill)
	if errDecode != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	cartTotalPrice, _ := strconv.ParseFloat(paymentBill.CartTotalPrice, 64)

	transactionChargeReq := &coreapi.ChargeReq{
		PaymentType: coreapi.PaymentTypeQris,
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  cartID,
			GrossAmt: int64(cartTotalPrice),
		},
		CustomerDetails: &midtrans.CustomerDetails{
			FName: currentUser,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    cartID,
				Price: int64(cartTotalPrice),
				Qty:   1,
				Name:  paymentBill.ProductName,
			},
		},
	}

	res, err := coreAPI.ChargeTransaction(transactionChargeReq)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(currentUser)
	json.NewEncoder(w).Encode(res)
}

func GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	cartID := params["cart_id"]

	var cart Cart
	res, err := coreAPI.CheckTransaction(cartID)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
	json.NewEncoder(w).Encode(res)
}
