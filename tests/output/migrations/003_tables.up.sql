create table "factory"."machines" (
    "name" text not null,
    "toys_produced" bigint not null
);


create table "warehouse"."storage_locations" (
    "shelf" bigint not null,
    "total_capacity" bigint not null,
    "used_capacity" bigint not null,
    "current_toy_type" text not null
);
