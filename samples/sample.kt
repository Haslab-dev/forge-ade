package com.example

class User(val name: String)
data class Pair(val a: Int, val b: Int)
enum class Color { RED, GREEN }

interface Repo {
    fun get(id: Int): User
}

typealias Callback = (Int) -> Unit
const val LIMIT = 5
val appName = "forge"

fun main() {}
fun User.greet(): String = ""
