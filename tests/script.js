import http from 'k6/http';
import { check } from 'k6';

export const options = {
};

const BASE_URL = "http://localhost:8000/v1"

export default function () {
  // Create two accounts
  let name = "Account 1"
  let res = http.post(`${BASE_URL}/accounts`, JSON.stringify({ name }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { "Account 1 created": (res) => res.status === 201 });
  let account_id_1 = res.json().id;

  name = "Account 2"
  res = http.post(`${BASE_URL}/accounts`, JSON.stringify({ name }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { "Account 2 created": (res) => res.status === 201 });
  let account_id_2 = res.json().id;

  // Charge account 1 with BTC
  res = http.post(`${BASE_URL}/accounts/${account_id_1}/charge`, JSON.stringify({ amount: 100, asset_code: "BTC" }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { "Charged account 1 with BTC": (res) => res.status === 200 });

  // Charge account 2 with 500.000 BRL
  res = http.post(`${BASE_URL}/accounts/${account_id_2}/charge`, JSON.stringify({ amount: 500000, asset_code: "BRL" }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { "Charged account 2 with BRL": (res) => res.status === 200 });

  // Create multiple sell and buy orders
  for (let i = 0; i < 10; i++) {
    let payload = {
      "account_id": account_id_1,
      "asset_code": "BTC",
      "quantity": 1,
      "price": 100,
      "order_type": "sell",
    }
    res = http.post(`${BASE_URL}/order_book`, JSON.stringify(payload), { headers: { 'Content-Type': 'application/json' } });
    check(res, { "Created first sell order": (res) => res.status === 204 });
  }

  for (let i = 0; i < 10; i++) {
    let payload = {
      "account_id": account_id_2,
      "asset_code": "BTC",
      "quantity": 1,
      "price": 100,
      "order_type": "buy",
    }
    res = http.post(`${BASE_URL}/order_book`, JSON.stringify(payload), { headers: { 'Content-Type': 'application/json' } });
    check(res, { "Created first buy order": (res) => res.status === 204 });
  }

  // Check account balances
  checkAccountBalances(account_id_1, "90", "1000");
  checkAccountBalances(account_id_2, "10", "499000");

  // Create a big sell order
  let payload = {
    "account_id": account_id_1,
    "asset_code": "BTC",
    "quantity": 50,
    "price": 100,
    "order_type": "sell",
  }
  res = http.post(`${BASE_URL}/order_book`, JSON.stringify(payload), { headers: { 'Content-Type': 'application/json' } });
  check(res, { "Created big sell order": (res) => res.status === 204 });

  // Buy then in small orders
  for (let i = 0; i < 100; i++) {
    let payload = {
      "account_id": account_id_2,
      "asset_code": "BTC",
      "quantity": 0.5,
      "price": 100,
      "order_type": "buy",
    }
    res = http.post(`${BASE_URL}/order_book`, JSON.stringify(payload), { headers: { 'Content-Type': 'application/json' } });
    check(res, { "Created small buy order": (res) => res.status === 204 });
  }

  // Check account balances
  checkAccountBalances(account_id_1, "40", "6000");
  checkAccountBalances(account_id_2, "60", "494000");
};

function checkAccountBalances(account_id, expected_btc_balance, expected_brl_balance) {
  let res = http.get(`${BASE_URL}/accounts/${account_id}`);
  check(res, { "status is 200": (res) => res.status === 200 });
  res.json().balances.forEach((el) => {
    if (el["asset_code"] === "BTC") {
      check(el, { "BTC balance is correct": (el) => el["balance"] === expected_btc_balance });
    } else if (el["asset_code"] === "BRL") {
      check(el, { "BRL balance is correct": (el) => el["balance"] === expected_brl_balance });
    }
  });
}