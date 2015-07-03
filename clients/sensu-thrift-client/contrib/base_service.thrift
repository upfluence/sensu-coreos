namespace py base.base_service

enum status {
  DEAD = 0,
  STARTING = 1,
  ALIVE = 2,
  STOPPING = 3,
  STOPPED = 4,
  WARNING = 5,
}

service BaseService {
  string getName(),
  string getVersion(),
  status getStatus(),
  i64 aliveSince(),
}
