import org.apache.spark.SparkContext
import org.apache.spark.SparkContext._
import org.apache.spark.SparkConf
import org.apache.spark.sql.SQLContext

object SimpleApp {
  def main(args: Array[String]) {

val cl = ClassLoader.getSystemClassLoader()
    // This is the hardcoded url to a mysql slave, you can also aggregate
    // multiple servers under a single label and use that hostname instead
    val mysqlHostname = "di-mysql-dbs-2.di"

    val dbName = "wordpress"
    val dbTable = "wp_comments"
    val dbUser = "wordpress"
    val dbPass = "wordpress"

    val conf = new SparkConf().setAppName("CountComments")
    val sc = new SparkContext(conf)
    val sqlcontext = new org.apache.spark.sql.SQLContext(sc)

    val comments = sqlcontext.read.format("jdbc").option("url", "jdbc:mysql://%s:3306/%s".format(mysqlHostname, dbName)).option("driver", "com.mysql.jdbc.Driver").option("dbtable", dbTable).option("user", dbUser).option("password", dbPass).load()

    val nComments = comments.count()

    println("There are %d comment(s)".format(nComments))
  }
}
