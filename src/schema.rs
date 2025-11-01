// @generated automatically by Diesel CLI.

pub mod sql_types {
    #[derive(diesel::sql_types::SqlType)]
    #[diesel(postgres_type(name = "delivery_type"))]
    pub struct DeliveryType;

    #[derive(diesel::sql_types::SqlType)]
    #[diesel(postgres_type(name = "order_status"))]
    pub struct OrderStatus;

    #[derive(diesel::sql_types::SqlType)]
    #[diesel(postgres_type(name = "payment_status"))]
    pub struct PaymentStatus;

    #[derive(diesel::sql_types::SqlType)]
    #[diesel(postgres_type(name = "product_status"))]
    pub struct ProductStatus;

    #[derive(diesel::sql_types::SqlType)]
    #[diesel(postgres_type(name = "user_role"))]
    pub struct UserRole;
}

diesel::table! {
    cart (cart_id) {
        cart_id -> Int8,
        user_id -> Int8,
        created_at -> Timestamp,
    }
}

diesel::table! {
    cart_items (item_id) {
        item_id -> Int8,
        cart_id -> Int8,
        variant_id -> Int8,
        quantity -> Nullable<Int4>,
    }
}

diesel::table! {
    favorites (user_id, product_id) {
        user_id -> Int8,
        product_id -> Int8,
        created_at -> Timestamp,
    }
}

diesel::table! {
    order_items (order_item_id) {
        order_item_id -> Int8,
        order_id -> Int8,
        variant_id -> Int8,
        quantity -> Nullable<Int4>,
        unit_price -> Nullable<Numeric>,
    }
}

diesel::table! {
    use diesel::sql_types::*;
    use super::sql_types::OrderStatus;
    use super::sql_types::DeliveryType;

    orders (order_id) {
        order_id -> Int8,
        user_id -> Int8,
        total_amount -> Nullable<Numeric>,
        order_status -> OrderStatus,
        delivery_type -> Nullable<DeliveryType>,
        shipping_address -> Nullable<Text>,
        #[max_length = 100]
        tracking_number -> Nullable<Varchar>,
        created_at -> Timestamp,
    }
}

diesel::table! {
    use diesel::sql_types::*;
    use super::sql_types::PaymentStatus;

    payments (payment_id) {
        payment_id -> Int8,
        order_id -> Int8,
        amount_paid -> Nullable<Numeric>,
        #[max_length = 255]
        slip_url -> Nullable<Varchar>,
        payment_status -> PaymentStatus,
        created_at -> Timestamp,
    }
}

diesel::table! {
    use diesel::sql_types::*;
    use super::sql_types::ProductStatus;

    products (product_id) {
        product_id -> Int8,
        #[max_length = 150]
        name -> Varchar,
        description -> Nullable<Text>,
        base_price -> Numeric,
        status -> ProductStatus,
    }
}

diesel::table! {
    user_addresses (address_id) {
        address_id -> Int8,
        user_id -> Int8,
        address -> Nullable<Text>,
        is_default -> Bool,
    }
}

diesel::table! {
    use diesel::sql_types::*;
    use super::sql_types::UserRole;

    users (user_id) {
        user_id -> Int8,
        #[max_length = 100]
        full_name -> Nullable<Varchar>,
        #[max_length = 100]
        email -> Varchar,
        #[max_length = 255]
        password_hash -> Varchar,
        #[max_length = 20]
        phone -> Nullable<Varchar>,
        role -> UserRole,
        created_at -> Timestamp,
        email_verified -> Bool,
    }
}

diesel::table! {
    variants (variant_id) {
        variant_id -> Int8,
        product_id -> Int8,
        #[max_length = 20]
        size -> Nullable<Varchar>,
        #[max_length = 50]
        color -> Nullable<Varchar>,
        stock_quantity -> Nullable<Int4>,
    }
}

diesel::joinable!(cart -> users (user_id));
diesel::joinable!(cart_items -> cart (cart_id));
diesel::joinable!(cart_items -> variants (variant_id));
diesel::joinable!(favorites -> products (product_id));
diesel::joinable!(favorites -> users (user_id));
diesel::joinable!(order_items -> orders (order_id));
diesel::joinable!(order_items -> variants (variant_id));
diesel::joinable!(orders -> users (user_id));
diesel::joinable!(payments -> orders (order_id));
diesel::joinable!(user_addresses -> users (user_id));
diesel::joinable!(variants -> products (product_id));

diesel::allow_tables_to_appear_in_same_query!(
    cart,
    cart_items,
    favorites,
    order_items,
    orders,
    payments,
    products,
    user_addresses,
    users,
    variants,
);
