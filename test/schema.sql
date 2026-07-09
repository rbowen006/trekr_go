--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14 (Debian 16.14-1.pgdg12+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: active_storage_attachments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.active_storage_attachments (
    id bigint NOT NULL,
    name character varying NOT NULL,
    record_type character varying NOT NULL,
    record_id bigint NOT NULL,
    blob_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL
);


--
-- Name: active_storage_attachments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.active_storage_attachments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: active_storage_attachments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.active_storage_attachments_id_seq OWNED BY public.active_storage_attachments.id;


--
-- Name: active_storage_blobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.active_storage_blobs (
    id bigint NOT NULL,
    key character varying NOT NULL,
    filename character varying NOT NULL,
    content_type character varying,
    metadata text,
    service_name character varying NOT NULL,
    byte_size bigint NOT NULL,
    checksum character varying,
    created_at timestamp(6) without time zone NOT NULL
);


--
-- Name: active_storage_blobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.active_storage_blobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: active_storage_blobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.active_storage_blobs_id_seq OWNED BY public.active_storage_blobs.id;


--
-- Name: active_storage_variant_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.active_storage_variant_records (
    id bigint NOT NULL,
    blob_id bigint NOT NULL,
    variation_digest character varying NOT NULL
);


--
-- Name: active_storage_variant_records_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.active_storage_variant_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: active_storage_variant_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.active_storage_variant_records_id_seq OWNED BY public.active_storage_variant_records.id;


--
-- Name: ai_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_requests (
    id bigint NOT NULL,
    feature character varying NOT NULL,
    model character varying NOT NULL,
    prompt_version character varying,
    input_tokens integer,
    output_tokens integer,
    latency_ms integer,
    estimated_cost_usd numeric(10,6),
    success boolean DEFAULT false NOT NULL,
    error_message character varying,
    request_payload text,
    response_payload text,
    user_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_requests_id_seq OWNED BY public.ai_requests.id;


--
-- Name: ar_internal_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ar_internal_metadata (
    key character varying NOT NULL,
    value character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: bookings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bookings (
    id bigint NOT NULL,
    start_date date,
    end_date date,
    status character varying DEFAULT 'pending'::character varying NOT NULL,
    hirer_id integer NOT NULL,
    rv_listing_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: bookings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bookings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bookings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bookings_id_seq OWNED BY public.bookings.id;


--
-- Name: chats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chats (
    id bigint NOT NULL,
    hirer_id integer NOT NULL,
    owner_id integer NOT NULL,
    rv_listing_id integer,
    booking_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    last_message_at timestamp(6) without time zone,
    last_message_content text,
    hirer_last_read_at timestamp(6) without time zone,
    owner_last_read_at timestamp(6) without time zone
);


--
-- Name: chats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chats_id_seq OWNED BY public.chats.id;


--
-- Name: knowledge_chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_chunks (
    id bigint NOT NULL,
    region character varying NOT NULL,
    heading character varying,
    content text NOT NULL,
    embedding public.vector(768),
    model character varying,
    content_hash character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: knowledge_chunks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_chunks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_chunks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_chunks_id_seq OWNED BY public.knowledge_chunks.id;


--
-- Name: listing_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.listing_embeddings (
    id bigint NOT NULL,
    rv_listing_id bigint NOT NULL,
    embedding public.vector(768),
    document text,
    model character varying,
    content_hash character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: listing_embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.listing_embeddings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: listing_embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.listing_embeddings_id_seq OWNED BY public.listing_embeddings.id;


--
-- Name: messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.messages (
    id bigint NOT NULL,
    content text,
    user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    chat_id integer NOT NULL,
    read_at timestamp(6) without time zone
);


--
-- Name: messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.messages_id_seq OWNED BY public.messages.id;


--
-- Name: rv_listings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rv_listings (
    id bigint NOT NULL,
    title character varying,
    description text,
    price_per_day numeric,
    owner_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    max_guests integer DEFAULT 1 NOT NULL,
    pet_friendly boolean DEFAULT false NOT NULL,
    latitude double precision,
    longitude double precision,
    rv_type integer DEFAULT 0 NOT NULL,
    town character varying,
    state character varying,
    postcode character varying,
    region character varying
);


--
-- Name: rv_listings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rv_listings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rv_listings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rv_listings_id_seq OWNED BY public.rv_listings.id;


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: trip_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trip_plans (
    id bigint NOT NULL,
    booking_id bigint NOT NULL,
    status character varying DEFAULT 'pending'::character varying NOT NULL,
    interests text,
    itinerary jsonb,
    error text,
    input_hash character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: trip_plans_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.trip_plans_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: trip_plans_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.trip_plans_id_seq OWNED BY public.trip_plans.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    email character varying DEFAULT ''::character varying NOT NULL,
    encrypted_password character varying DEFAULT ''::character varying NOT NULL,
    reset_password_token character varying,
    reset_password_sent_at timestamp(6) without time zone,
    remember_created_at timestamp(6) without time zone,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: active_storage_attachments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_attachments ALTER COLUMN id SET DEFAULT nextval('public.active_storage_attachments_id_seq'::regclass);


--
-- Name: active_storage_blobs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_blobs ALTER COLUMN id SET DEFAULT nextval('public.active_storage_blobs_id_seq'::regclass);


--
-- Name: active_storage_variant_records id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_variant_records ALTER COLUMN id SET DEFAULT nextval('public.active_storage_variant_records_id_seq'::regclass);


--
-- Name: ai_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_requests ALTER COLUMN id SET DEFAULT nextval('public.ai_requests_id_seq'::regclass);


--
-- Name: bookings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings ALTER COLUMN id SET DEFAULT nextval('public.bookings_id_seq'::regclass);


--
-- Name: chats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats ALTER COLUMN id SET DEFAULT nextval('public.chats_id_seq'::regclass);


--
-- Name: knowledge_chunks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks ALTER COLUMN id SET DEFAULT nextval('public.knowledge_chunks_id_seq'::regclass);


--
-- Name: listing_embeddings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.listing_embeddings ALTER COLUMN id SET DEFAULT nextval('public.listing_embeddings_id_seq'::regclass);


--
-- Name: messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages ALTER COLUMN id SET DEFAULT nextval('public.messages_id_seq'::regclass);


--
-- Name: rv_listings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rv_listings ALTER COLUMN id SET DEFAULT nextval('public.rv_listings_id_seq'::regclass);


--
-- Name: trip_plans id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_plans ALTER COLUMN id SET DEFAULT nextval('public.trip_plans_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: active_storage_attachments active_storage_attachments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_attachments
    ADD CONSTRAINT active_storage_attachments_pkey PRIMARY KEY (id);


--
-- Name: active_storage_blobs active_storage_blobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_blobs
    ADD CONSTRAINT active_storage_blobs_pkey PRIMARY KEY (id);


--
-- Name: active_storage_variant_records active_storage_variant_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_variant_records
    ADD CONSTRAINT active_storage_variant_records_pkey PRIMARY KEY (id);


--
-- Name: ai_requests ai_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_requests
    ADD CONSTRAINT ai_requests_pkey PRIMARY KEY (id);


--
-- Name: ar_internal_metadata ar_internal_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ar_internal_metadata
    ADD CONSTRAINT ar_internal_metadata_pkey PRIMARY KEY (key);


--
-- Name: bookings bookings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_pkey PRIMARY KEY (id);


--
-- Name: chats chats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_pkey PRIMARY KEY (id);


--
-- Name: knowledge_chunks knowledge_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT knowledge_chunks_pkey PRIMARY KEY (id);


--
-- Name: listing_embeddings listing_embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.listing_embeddings
    ADD CONSTRAINT listing_embeddings_pkey PRIMARY KEY (id);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: rv_listings rv_listings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rv_listings
    ADD CONSTRAINT rv_listings_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: trip_plans trip_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_plans
    ADD CONSTRAINT trip_plans_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: index_active_storage_attachments_on_blob_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_active_storage_attachments_on_blob_id ON public.active_storage_attachments USING btree (blob_id);


--
-- Name: index_active_storage_attachments_uniqueness; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_active_storage_attachments_uniqueness ON public.active_storage_attachments USING btree (record_type, record_id, name, blob_id);


--
-- Name: index_active_storage_blobs_on_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_active_storage_blobs_on_key ON public.active_storage_blobs USING btree (key);


--
-- Name: index_active_storage_variant_records_uniqueness; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_active_storage_variant_records_uniqueness ON public.active_storage_variant_records USING btree (blob_id, variation_digest);


--
-- Name: index_ai_requests_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_requests_on_user_id ON public.ai_requests USING btree (user_id);


--
-- Name: index_bookings_on_hirer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_bookings_on_hirer_id ON public.bookings USING btree (hirer_id);


--
-- Name: index_bookings_on_rv_listing_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_bookings_on_rv_listing_id ON public.bookings USING btree (rv_listing_id);


--
-- Name: index_chats_on_booking_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_booking_id ON public.chats USING btree (booking_id);


--
-- Name: index_chats_on_hirer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_hirer_id ON public.chats USING btree (hirer_id);


--
-- Name: index_chats_on_hirer_id_and_last_message_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_hirer_id_and_last_message_at ON public.chats USING btree (hirer_id, last_message_at);


--
-- Name: index_chats_on_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_owner_id ON public.chats USING btree (owner_id);


--
-- Name: index_chats_on_owner_id_and_last_message_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_owner_id_and_last_message_at ON public.chats USING btree (owner_id, last_message_at);


--
-- Name: index_chats_on_rv_listing_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chats_on_rv_listing_id ON public.chats USING btree (rv_listing_id);


--
-- Name: index_knowledge_chunks_on_region; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_knowledge_chunks_on_region ON public.knowledge_chunks USING btree (region);


--
-- Name: index_knowledge_chunks_on_region_and_content_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_knowledge_chunks_on_region_and_content_hash ON public.knowledge_chunks USING btree (region, content_hash);


--
-- Name: index_listing_embeddings_on_rv_listing_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_listing_embeddings_on_rv_listing_id ON public.listing_embeddings USING btree (rv_listing_id);


--
-- Name: index_messages_on_chat_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_messages_on_chat_id ON public.messages USING btree (chat_id);


--
-- Name: index_messages_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_messages_on_user_id ON public.messages USING btree (user_id);


--
-- Name: index_rv_listings_on_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_rv_listings_on_owner_id ON public.rv_listings USING btree (owner_id);


--
-- Name: index_trip_plans_on_booking_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_trip_plans_on_booking_id ON public.trip_plans USING btree (booking_id);


--
-- Name: index_users_on_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_email ON public.users USING btree (email);


--
-- Name: index_users_on_reset_password_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_reset_password_token ON public.users USING btree (reset_password_token);


--
-- Name: bookings fk_rails_0bbdc70b73; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT fk_rails_0bbdc70b73 FOREIGN KEY (rv_listing_id) REFERENCES public.rv_listings(id);


--
-- Name: messages fk_rails_0f670de7ba; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT fk_rails_0f670de7ba FOREIGN KEY (chat_id) REFERENCES public.chats(id);


--
-- Name: messages fk_rails_273a25a7a6; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT fk_rails_273a25a7a6 FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: chats fk_rails_2fd6dc4e9d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT fk_rails_2fd6dc4e9d FOREIGN KEY (booking_id) REFERENCES public.bookings(id);


--
-- Name: chats fk_rails_3d34334e72; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT fk_rails_3d34334e72 FOREIGN KEY (rv_listing_id) REFERENCES public.rv_listings(id);


--
-- Name: bookings fk_rails_5cec5aa3b9; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT fk_rails_5cec5aa3b9 FOREIGN KEY (hirer_id) REFERENCES public.users(id);


--
-- Name: ai_requests fk_rails_6978dd624a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_requests
    ADD CONSTRAINT fk_rails_6978dd624a FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: active_storage_variant_records fk_rails_993965df05; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_variant_records
    ADD CONSTRAINT fk_rails_993965df05 FOREIGN KEY (blob_id) REFERENCES public.active_storage_blobs(id);


--
-- Name: listing_embeddings fk_rails_9dd3587eb9; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.listing_embeddings
    ADD CONSTRAINT fk_rails_9dd3587eb9 FOREIGN KEY (rv_listing_id) REFERENCES public.rv_listings(id);


--
-- Name: chats fk_rails_a229e37047; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT fk_rails_a229e37047 FOREIGN KEY (owner_id) REFERENCES public.users(id);


--
-- Name: chats fk_rails_b590147250; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT fk_rails_b590147250 FOREIGN KEY (hirer_id) REFERENCES public.users(id);


--
-- Name: rv_listings fk_rails_bda34cb64d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rv_listings
    ADD CONSTRAINT fk_rails_bda34cb64d FOREIGN KEY (owner_id) REFERENCES public.users(id);


--
-- Name: active_storage_attachments fk_rails_c3b3935057; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.active_storage_attachments
    ADD CONSTRAINT fk_rails_c3b3935057 FOREIGN KEY (blob_id) REFERENCES public.active_storage_blobs(id);


--
-- Name: trip_plans fk_rails_eb558d7ac1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_plans
    ADD CONSTRAINT fk_rails_eb558d7ac1 FOREIGN KEY (booking_id) REFERENCES public.bookings(id);


--
-- PostgreSQL database dump complete
--


