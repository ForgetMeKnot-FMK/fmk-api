const regForm = document.querySelector("#reg_form");
const loginForm = document.querySelector("#login_form");

const API_BASE_URL = "http://localhost:8000/api/v1";

const b64enc = (bytes) => {
  return sodium.to_base64(bytes, sodium.base64_variants.ORIGINAL);
};
const b64dec = (encodedString) => {
  return sodium.from_base64(encodedString, sodium.base64_variants.ORIGINAL);
};

/**
 * Send a POST request to the API and get a Response back (typically in JSON)
 *
 * @param {String} endpoint API endpoint, must contain a leading slash (e.g. `/ping`)
 * @param {Object} payload  Plain JS Object, will be sent in JSON format
 *
 * @returns {Promise<Response>}
 */
const apiRequest = (endpoint, payload) => {
  return fetch(`${API_BASE_URL}${endpoint}`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
};

regForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const userEncryptionData = generateUserEncryptionData(
    regForm["password"].value || "",
  );
  const userData = {
    name: regForm["name"].value || "",
    username: regForm["username"].value || "",
    ...userEncryptionData,
  };

  const rawRegResponse = await apiRequest("/register", userData);

  if (!rawRegResponse || rawRegResponse.status != 201) {
    log("reg", "registration failed", "error");
    return;
  }

  const regResponse = await rawRegResponse.json();

  // TODO: Question - Should we do sth with regResponse?

  log("reg", "registration successful");
});

/**
 * Very simple logging, logs in both the browser console as well the specified DOM container
 * This function needs to be removed, just kept for quick tests
 *
 * @param {String} ctx Its either `reg` or `login`
 * @param {String} msg The message to log
 * @param {String} type Either `error` (for an error message) or undefined (for regular ones)
 */
const log = (ctx, msg, type) => {
  // Helper Func
  const appendOutput = (ctx, msg) => {
    const selector =
      ctx == "reg"
        ? "#reg_form_output"
        : ctx == "login"
          ? "#login_form_output"
          : null;
    if (!selector) return;
    document.querySelector(selector).innerHTML += msg + "<hr />";
  };

  if (!type) {
    appendOutput(ctx, msg);
    console.log(`[${ctx}] ${msg}`);
  }
  if (type == "error") {
    appendOutput(ctx, `[error] ${msg}`);
    console.error(`[${ctx}][error] ${msg}`);
  }
};

/**
 * Generates the keys, salts, hashes and IVs necessary to be computed by the client during registration
 *
 * @param {String} password Plaintext password, recieved from user input
 * @returns {Object} Encryption data pertaining to the user, generated client-side
 *
 *                    This includes the following (base64 encoded):
 *
 *                    (1) `authHash` (2) `kdf_salt` (3) `encrypted_master_key` and (4) `encrypted_master_key_iv`
 */
const generateUserEncryptionData = (password) => {
  /**
   * Step 1: Gather params and generate salt
   */
  const PASSWORD = String(password);
  const kdf_salt = sodium.randombytes_buf(sodium.crypto_pwhash_SALTBYTES); // 16 bytes
  const kdf_iterations = sodium.crypto_pwhash_OPSLIMIT_MODERATE;
  const kdf_mem_kib = sodium.crypto_pwhash_MEMLIMIT_MODERATE;
  const kdf_algo = sodium.crypto_pwhash_ALG_ARGON2ID13;

  // Key lengths in bytes
  const masterSecretLength = 32;
  const stretchedKEKLength = 32;
  const authKeyLength = 32;
  const masterKeyLength = 32;

  /**
   * Step 2: Generate the master secret
   */
  const masterSecret = sodium.crypto_pwhash(
    masterSecretLength,
    PASSWORD,
    kdf_salt,
    kdf_iterations,
    kdf_mem_kib,
    kdf_algo,
  );

  /**
   * Step 3: Make the split role-specific keys (enc, auth)
   */
  const stretchedKEK = sodium.crypto_kdf_derive_from_key(
    stretchedKEKLength,
    1,
    "fmk_encr",
    masterSecret,
  );
  const authKey = sodium.crypto_kdf_derive_from_key(
    authKeyLength,
    2,
    "fmk_auth",
    masterSecret,
  );

  /**
   * Step 4: Generate the master key (not derived from anything else)
   */
  const masterKey = sodium.randombytes_buf(masterKeyLength);

  /**
   * Step 5: Wrap the master key for storage
   */
  const encrypted_master_key_iv = sodium.randombytes_buf(
    sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES,
  );
  const encrypted_master_key =
    sodium.crypto_aead_xchacha20poly1305_ietf_encrypt(
      masterKey,
      null,
      null,
      encrypted_master_key_iv,
      stretchedKEK,
    );

  /**
   * Step 6: Generate auth key hash (for logins)
   */
  const authHash = sodium.crypto_hash_sha256(authKey);

  /**
   * Step 7: Generate asymmetric keypair (X25519)
   */
  const { publicKey: pub_key_x25519, privateKey: priv_key_x25519 } =
    sodium.crypto_box_keypair(); // both 32 bytes

  // TODO: send pub_key_x25519 to server; wrap priv_key_x25519 under masterKey (next step, later)

  /**
   * Step 8: Wrap the private key for storage
   */
  const encrypted_priv_key_iv = sodium.randombytes_buf(
    sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES,
  );
  const encrypted_priv_key = sodium.crypto_aead_xchacha20poly1305_ietf_encrypt(
    priv_key_x25519,
    null,
    null,
    encrypted_priv_key_iv,
    stretchedKEK,
  );

  const generatedData = {
    authHash,
    kdf_salt,
    encrypted_master_key,
    encrypted_master_key_iv,
    pub_key_x25519,
    encrypted_priv_key,
    encrypted_priv_key_iv,
  };

  for (const [k, v] of Object.entries(generatedData)) {
    generatedData[k] = b64enc(v);
  }

  return generatedData;
};

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();

  const username = loginForm["username"].value || "";
  const password = loginForm["password"].value || "";
  const formDataObject = {
    username,
  };

  const rawPreloginResponse = await apiRequest("/prelogin", formDataObject);

  if (!rawPreloginResponse || rawPreloginResponse.status != 200) {
    log("login", "user not found", "error");
    return;
  }

  const preLoginResponse = await rawPreloginResponse.json();

  log("login", "prelogin complete, procedding with next step");

  const masterSecret = generateMasterSecret(
    password,
    b64dec(preLoginResponse.kdf_salt),
    preLoginResponse.kdf_iterations,
    preLoginResponse.kdf_mem_kib,
    preLoginResponse.kdf_algo,
  );

  const authHash = generateAuthHash(masterSecret);

  const authHashEncoded = b64enc(authHash);

  log("login", "hash computed, attempting login");

  const loginData = {
    username,
    authHash: authHashEncoded,
  };

  const rawLoginResponse = await apiRequest("/login", loginData);

  if (!rawLoginResponse || rawLoginResponse.status != 200) {
    log("login", "wrong credentials", "error");
    return;
  }

  const loginResponse = await rawLoginResponse.json();

  // TODO: Process successful login (do sth with loginResponse)

  log("login", "login successful");
  log("login", JSON.stringify(loginResponse, null, 2));
  log("login", "Unsealing session ID...");

  // Decrypt the private key
  const stretchedKEKLength = 32;
  const stretchedKEK = sodium.crypto_kdf_derive_from_key(
    stretchedKEKLength,
    1,
    "fmk_encr",
    masterSecret,
  );
  const priv_key_x25519 = sodium.crypto_aead_xchacha20poly1305_ietf_decrypt(
    null,
    b64dec(loginResponse.encrypted_priv_key),
    null,
    b64dec(loginResponse.encrypted_priv_key_iv),
    stretchedKEK,
  );

  const unsealed_session_id = sodium.crypto_box_seal_open(
    b64dec(loginResponse.sealed_session_id),
    b64dec(loginResponse.pub_key_x25519),
    priv_key_x25519,
  );

  // TODO: Store this as a cookie with:
  // `HttpOnly, Secure, SameSite=Lax (or Strict), and Path=/
  log(unsealed_session_id);
});

/**
 * Generate the deterministic master secret from a password and KDF parameters
 *
 * @param {String} password Plaintext password, recieved from user input
 * @param {Uint8Array} kdf_salt Salt to be used for secret generation
 * @param {Number} kdf_iterations KDF Iterations to use (e.g. `sodium.crypto_pwhash_OPSLIMIT_MODERATE`)
 * @param {Number} kdf_mem_kib Memory limit for KDF in KB (e.g. `sodium.crypto_pwhash_MEMLIMIT_MODERATE`)
 * @param {Number} kdf_algo Algorithm to use (enum value, e.g. `sodium.crypto_pwhash_ALG_ARGON2ID13`)
 *
 * @returns {Uint8Array} The client-side master secret
 */
const generateMasterSecret = (
  password,
  kdf_salt,
  kdf_iterations,
  kdf_mem_kib,
  kdf_algo,
) => {
  // Key length in bytes
  const masterSecretLength = 32;

  const masterSecret = sodium.crypto_pwhash(
    masterSecretLength,
    password,
    kdf_salt,
    kdf_iterations,
    kdf_mem_kib,
    kdf_algo,
  );

  return masterSecret;
};

/**
 * Generate the hash of the auth key from the master secret
 *
 * @param {Uint8Array} The deterministic client-side master secret
 *
 * @returns {Uint8Array} A 32-byte SHA-256 hash of the `authKey`
 */
const generateAuthHash = (masterSecret) => {
  // Key lengths in bytes
  const authKeyLength = 32;

  const authKey = sodium.crypto_kdf_derive_from_key(
    authKeyLength,
    2,
    "fmk_auth",
    masterSecret,
  );

  const authHash = sodium.crypto_hash_sha256(authKey);

  return authHash;
};
