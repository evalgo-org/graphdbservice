from pydantic import ConfigDict
from pydantic_settings import BaseSettings

################################
# Security Config
################################


class Settings(BaseSettings):
    DBUSER: str = ""
    DBPW: str = ""
    COMMON_TIMEOUT: int = 400
    METASELECT_BASE_URL: str = "https://metaselect-dev.cfapps.eu10.hana.ondemand.com"
    METASELECT_API_KEY: str = "" 
    GRAPHDBBASEURL:str = "http://ke-test.graphdb.px:7200/repositories/"
    REPOSTAGINGEG: str = "ProductData-MDM-keys-EG"
    REPOSTAGINGUS: str = "ProductData-MDM-keys-US"
    MAINTENANCE: str = "Maintenance-001"
    model_config = ConfigDict(extra='allow',env_file = ".env")

settings = Settings()
