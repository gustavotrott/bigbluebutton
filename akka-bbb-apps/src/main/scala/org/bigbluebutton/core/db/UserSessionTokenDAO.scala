package org.bigbluebutton.core.db
import slick.jdbc.PostgresProfile.api._

import scala.concurrent.ExecutionContext.Implicits.global
import scala.util.{Failure, Success }

case class UserSessionTokenDbModel (
       meetingId:               String,
       userId:                  String,
       sessionToken:            String,
       isPrimarySession:        Boolean,
       createdAt:               java.sql.Timestamp,
       removedAt:               Option[java.sql.Timestamp],
)


class UserSessionTokenDbTableDef(tag: Tag) extends Table[UserSessionTokenDbModel](tag, None, "user_sessionToken") {
  override def * = (
    meetingId, userId, sessionToken, isPrimarySession, createdAt, removedAt
  ) <> (UserSessionTokenDbModel.tupled, UserSessionTokenDbModel.unapply)
  val meetingId = column[String]("meetingId", O.PrimaryKey)
  val userId = column[String]("userId", O.PrimaryKey)
  val sessionToken = column[String]("sessionToken", O.PrimaryKey)
  val isPrimarySession = column[Boolean]("isPrimarySession")
  val createdAt = column[java.sql.Timestamp]("createdAt")
  val removedAt = column[Option[java.sql.Timestamp]]("removedAt")
}


object UserSessionTokenDAO {
  def insert(meetingId: String, userId: String, sessionToken: String, isPrimarySession:Boolean) = {
    DatabaseConnection.db.run(
      TableQuery[UserSessionTokenDbTableDef].insertOrUpdate(
        UserSessionTokenDbModel(
          meetingId = meetingId,
          userId = userId,
          sessionToken = sessionToken,
          isPrimarySession = isPrimarySession,
          createdAt = new java.sql.Timestamp(System.currentTimeMillis()),
          removedAt = None
        )
      )
    ).onComplete {
        case Success(rowsAffected) => {
          DatabaseConnection.logger.debug(s"$rowsAffected row(s) inserted on user_sessionToken table!")
        }
        case Failure(e)            => DatabaseConnection.logger.debug(s"Error inserting user_sessionToken: $e")
      }
  }

  def softDelete(meetingId: String, userId: String, sessionToken: String) = {
    DatabaseConnection.db.run(
      TableQuery[UserSessionTokenDbTableDef]
        .filter(_.meetingId === meetingId)
        .filter(_.userId === userId)
        .filter(_.sessionToken === sessionToken)
        .filter(_.removedAt.isEmpty)
        .map(u => u.removedAt)
        .update(Some(new java.sql.Timestamp(System.currentTimeMillis())))
    ).onComplete {
      case Success(rowsAffected) => DatabaseConnection.logger.debug(s"$rowsAffected row(s) updated on user_sessionToken table!")
      case Failure(e) => DatabaseConnection.logger.error(s"Error updating user_sessionToken: $e")
    }
  }


}
