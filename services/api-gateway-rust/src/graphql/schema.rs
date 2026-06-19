use crate::graphql::resolvers::{Mutation, Query};
use async_graphql::{EmptySubscription, Schema};

pub type AppSchema = Schema<Query, Mutation, EmptySubscription>;

pub fn create_schema() -> AppSchema {
    Schema::build(Query, Mutation, EmptySubscription).finish()
}
