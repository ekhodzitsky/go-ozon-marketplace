use crate::clients::Clients;
use crate::graphql::resolvers::{Mutation, Query};
use crate::graphql::subscription::SubscriptionRoot;
use async_graphql::Schema;

pub type AppSchema = Schema<Query, Mutation, SubscriptionRoot>;

pub fn create_schema(
    redis_pool: deadpool_redis::Pool,
    redis_client: redis::Client,
    clients: Clients,
    introspection_enabled: bool,
) -> AppSchema {
    let mut builder = Schema::build(Query, Mutation, SubscriptionRoot)
        .data(redis_pool)
        .data(redis_client)
        .data(clients);
    if !introspection_enabled {
        builder = builder.disable_introspection();
    }
    builder.finish()
}
