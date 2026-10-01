-- Google Play purchase tokens run well past the VARCHAR(100) that
-- payments.transaction_id was sized at for Midtrans transaction ids (~135
-- chars observed, and Google documents no upper bound), so every Play
-- verification failed on insert with SQLSTATE 22001. The same column also
-- holds User Choice Billing externalTransactionTokens, which are equally
-- unbounded.
ALTER TABLE payments ALTER COLUMN transaction_id TYPE TEXT;

-- orders.purchase_token (V52) holds the same kind of token and its
-- VARCHAR(300) is a guess at the same unbounded value — widen it too rather
-- than wait for a longer token to hit it.
ALTER TABLE orders ALTER COLUMN purchase_token TYPE TEXT;
