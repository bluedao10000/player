-- One row per outbound HTTP attempt, independent of business transactions.
CREATE TABLE model_calls (
 id uuid PRIMARY KEY, seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 purpose text NOT NULL CHECK (purpose IN ('content','image','vision','match')),
 operation text NOT NULL, subject text NOT NULL, endpoint text NOT NULL,
 provider_account text NOT NULL, cost_owner text NOT NULL DEFAULT 'platform', image_size text, image_quality text,
 requested_model text NOT NULL, resolved_model text, started_at timestamptz NOT NULL,
 finished_at timestamptz, outcome text NOT NULL CHECK(outcome IN ('started','ok','provider_error','http_error','unknown')),
 http_status integer, idempotency_key text, request_id text, response_id text, usage jsonb,
 input_tokens bigint CHECK(input_tokens >= 0), output_tokens bigint CHECK(output_tokens >= 0),
 total_tokens bigint CHECK(total_tokens >= 0), images integer CHECK(images >= 0),
 CHECK((outcome='started')=(finished_at IS NULL))
);
CREATE INDEX model_calls_by_request ON model_calls(endpoint,provider_account,request_id) WHERE request_id IS NOT NULL;
CREATE INDEX model_calls_by_time ON model_calls(started_at,id);
CREATE INDEX model_calls_by_subject ON model_calls(subject,started_at);
-- A provider may replay a receipt for an idempotent retry. Retain both attempts;
-- prefer the most complete observation of the receipt when summing consumption.
CREATE VIEW model_usage_receipts AS
 WITH request_alias AS (
   SELECT endpoint,provider_account,request_id,min(response_id) AS response_alias,
          count(DISTINCT response_id)>1 AS ambiguous_request
   FROM model_calls WHERE request_id IS NOT NULL
   GROUP BY endpoint,provider_account,request_id
 ), keyed AS (
   SELECT c.*,coalesce('response:'||c.response_id,'response:'||a.response_alias,
         'request:'||c.request_id,'attempt:'||c.id::text) receipt_key,
         coalesce(a.ambiguous_request,false) ambiguous_request
   FROM model_calls c LEFT JOIN request_alias a USING(endpoint,provider_account,request_id)
 ), ranked AS (
   SELECT k.*,min(started_at) OVER receipt AS receipt_started_at,
     bool_or(ambiguous_request OR (total_tokens IS NOT NULL AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL
       AND total_tokens::numeric<>input_tokens::numeric+output_tokens)) OVER receipt
      OR min(total_tokens) OVER receipt IS DISTINCT FROM max(total_tokens) OVER receipt
      OR min(input_tokens) OVER receipt IS DISTINCT FROM max(input_tokens) OVER receipt
      OR min(output_tokens) OVER receipt IS DISTINCT FROM max(output_tokens) OVER receipt
      OR min(images) OVER receipt IS DISTINCT FROM max(images) OVER receipt
      OR min(resolved_model) OVER receipt IS DISTINCT FROM max(resolved_model) OVER receipt AS conflict,
     row_number() OVER (receipt ORDER BY (total_tokens IS NOT NULL) DESC,
       (input_tokens IS NOT NULL AND output_tokens IS NOT NULL) DESC,(outcome='ok') DESC,started_at,id) receipt_rank
   FROM keyed k WINDOW receipt AS (PARTITION BY endpoint,provider_account,receipt_key)
 )
 SELECT r.*, CASE
   WHEN conflict THEN 'inconsistent'
   WHEN total_tokens IS NOT NULL THEN 'measured'
   WHEN input_tokens IS NOT NULL OR output_tokens IS NOT NULL THEN 'partial'
   WHEN outcome IN ('unknown','started') THEN 'unresolved'
   ELSE 'unreported' END AS meter_state
 FROM ranked r;
