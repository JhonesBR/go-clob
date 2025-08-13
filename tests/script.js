import http from 'k6/http';
import { check } from 'k6';

export const options = {
};

const BASE_URL = "http://localhost:8000/v1"
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default function () {
  // Create two accounts
  let account_id_1 = createAccount("Account 1");
  let account_id_2 = createAccount("Account 2");

  // Charge account 1 with BTC
  chargeAccount(account_id_1, 100, "BTC");

  // Charge account 2 with 500.000 BRL
  chargeAccount(account_id_2, 500000, "BRL");

  // Create multiple sell and buy orders
  for (let i = 0; i < 10; i++) {
    createOrder(account_id_1, "BTC", 1, 100, "sell");
  }
  for (let i = 0; i < 10; i++) {
    createOrder(account_id_2, "BTC", 1, 100, "buy");
  }

  // Check account balances
  checkAccountBalances(account_id_1, "90", "1000");
  checkAccountBalances(account_id_2, "10", "499000");

  // Create a big sell order
  createOrder(account_id_1, "BTC", 50, 100, "sell");

  // Buy then in small orders
  for (let i = 0; i < 100; i++) {
    createOrder(account_id_2, "BTC", 0.5, 100, "buy");
  }

  // Check account balances
  checkAccountBalances(account_id_1, "40", "6000");
  checkAccountBalances(account_id_2, "60", "494000");
};

function createAccount(name) {
  let res = http.post(`${BASE_URL}/accounts`, JSON.stringify({ name }), HEADERS);
  check(res, { [`Account ${name} created`]: (res) => res.status === 201 });
  return res.json().id;
}

function chargeAccount(account_id, amount, asset_code) {
  let res = http.post(`${BASE_URL}/accounts/${account_id}/charge`, JSON.stringify({ amount, asset_code }), HEADERS);
  check(res, { [`Charged account ${account_id} with ${asset_code}`]: (res) => res.status === 200 });
}

function createOrder(account_id, asset_code, quantity, price, order_type) {
  let payload = { account_id, asset_code, quantity, price, order_type };
  let res = http.post(`${BASE_URL}/order_book`, JSON.stringify(payload), HEADERS);
  check(res, { [`Created ${order_type} order`]: (res) => res.status === 204 });
}

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