"""Sample Python module for tool-calling tests.

Contents:
- Database class (host-based connect stub)
- make_user factory with default age
- async fetch stub
"""
import os

CONST_MAX = 100
app_name = "forge"

class Database:
    def __init__(self, host):
        self.host = host

    def connect(self):
        pass
      

def make_user(name, age=30):
    return {"name": name}

async def fetch(url):
    pass
