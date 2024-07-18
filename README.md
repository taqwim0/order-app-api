
# Order App API

A simple REST API about 1 happy flow for order application. Integrated with Midtrans API as 3rd party payment simulator.  Created using Go, Postgresql & Docker.


## Prerequisites

- Go
- Postgresql
- Docker
- Docker Compose


## Run Locally

* Clone the project

```bash
  git clone https://github.com/taqwim0/order-app-api
```

Go to the project directory

```bash
  cd order-app-api
```

Install dependencies

```bash
  docker-compose up -d
```
Put proper config on `app.env` file for fields

    JWT_SECRET

    MIDTRANS_SERVER_KEY

Midtrans documentation references: https://docs.midtrans.com/

Start the server

```bash
  go run main.go
```
You can try it on your `localhost:8000`

## API Reference

#### Login

```http
  POST /login
```

| Body | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `username` | `string` | username (required) |
| `password` | `string` | password (required) |

#### Welcome

```http
  GET /welcome
```

#### Refresh Token

```http
  GET /refresh
```

#### Get Products

```http
  GET /products
```

#### Add to Cart

```http
  POST /cart/add/{user_id}
```
| Parameter | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `user_id`      | `int` | user id (required) |

| Body | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `product_id`      | `int` | product id (required) |

#### Delete Cart

```http
  DEL /cart/delete/{cart_id}
```

| Parameter | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `cart_id`      | `int` | cart id (required) |

#### Get Cart

```http
  GET /cart/{user_id}
```

| Parameter | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `user_id`      | `int` | user id (required) |

#### Generate Payment Bill

```http
  POST /payment/bill/{cart_id}
```

| Parameter | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `cart_id`      | `int` | cart id (required) |

| Body | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `cart_total_price`      | `int` | total price (required) |

#### Get Payment Bill Status

```http
  GET /payment/status/{cart_id}
```

| Parameter | Type     | Description |
| :-------- | :------- | :-------------------------------- |
| `cart_id`      | `int` | cart id (required) |


